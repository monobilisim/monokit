//go:build linux

package mongodbHealth

import (
	"context"
	"fmt"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/healthdb"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// replSetMember mirrors the members[] entries in `replSetGetStatus` output.
type replSetMember struct {
	Name         string    `bson:"name"`
	Health       int32     `bson:"health"`
	StateStr     string    `bson:"stateStr"`
	OptimeDate   time.Time `bson:"optimeDate"`
	Self         bool      `bson:"self,omitempty"`
}

// replSetStatusResult mirrors the top-level `replSetGetStatus` output.
type replSetStatusResult struct {
	Set     string          `bson:"set"`
	Members []replSetMember `bson:"members"`
}

// IsReplicaSet returns true if the server responds to replSetGetStatus
// (i.e. is part of a replica set rather than a standalone). The returned
// error is non-nil whenever the check itself failed (network, auth, etc.)
// so the caller can distinguish "genuinely standalone" from "couldn't tell".
func IsReplicaSet(ctx context.Context, client *mongo.Client) (bool, error) {
	var result bson.M
	err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetGetStatus", Value: 1}}).Decode(&result)
	return err == nil, err
}

// CheckReplicaSet evaluates primary presence/change, secondary quorum,
// replication lag and the oplog window.
func CheckReplicaSet(ctx context.Context, client *mongo.Client) {
	var status replSetStatusResult
	err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "replSetGetStatus", Value: 1}}).Decode(&status)
	if err != nil {
		msg := fmt.Sprintf("Couldn't run replSetGetStatus: %s", err.Error())
		msgTr := fmt.Sprintf("replSetGetStatus çalıştırılamadı: %s", err.Error())
		subject := fmt.Sprintf("%s için MongoDB replica set durumu alınamadı", common.Config.Identifier)
		common.AlarmCheckDown("mongodb-connection", msg, false, "", "")
		issues.CheckDown("mongodb-connection", subject, msgTr, false, 0)
		return
	}
	common.AlarmCheckUp("mongodb-connection", "replSetGetStatus OK", false)

	healthData.ReplicaSet.SetName = status.Set

	primary, members := buildMembers(status.Members)
	healthData.ReplicaSet.Members = members

	checkPrimary(primary)
	checkSecondaryQuorum(members)
	checkReplicationLag(members)
	checkOplogWindow(ctx, client)
}

// buildMembers converts raw replSetMember entries into MemberInfo, and
// returns the primary member's name (empty if none).
func buildMembers(raw []replSetMember) (string, []MemberInfo) {
	var primary string
	members := make([]MemberInfo, 0, len(raw))

	for _, m := range raw {
		isPrimary := m.StateStr == "PRIMARY"
		if isPrimary {
			primary = m.Name
		}

		lagSeconds := time.Since(m.OptimeDate).Seconds()
		if isPrimary || lagSeconds < 0 {
			lagSeconds = 0
		}

		members = append(members, MemberInfo{
			Name:       m.Name,
			StateStr:   m.StateStr,
			Health:     int(m.Health),
			Healthy:    m.Health == 1,
			IsPrimary:  isPrimary,
			LagSeconds: lagSeconds,
		})
	}

	return primary, members
}

// checkPrimary detects primary absence and primary changes (informational).
func checkPrimary(primary string) {
	healthData.ReplicaSet.Primary = primary
	healthData.ReplicaSet.PrimaryAbsent = primary == ""

	if primary == "" {
		msg := "MongoDB replica set has no primary elected"
		msgTr := "MongoDB replica set için seçilmiş primary yok"
		subject := fmt.Sprintf("%s için MongoDB primary seçilemedi", common.Config.Identifier)
		common.AlarmCheckDown("mongodb-primary-absent", msg, false, "", "")
		issues.CheckDown("mongodb-primary-absent", subject, msgTr, false, 0)
		return
	}
	common.AlarmCheckUp("mongodb-primary-absent", "Primary is elected: "+primary, false)

	previous, _, _, found, err := healthdb.GetJSON("mongodbHealth", "primary")
	if err == nil && found && previous != "" && previous != primary {
		healthData.ReplicaSet.PreviousPrimary = previous
		healthData.ReplicaSet.PrimaryChanged = true

		message := fmt.Sprintf("[mongodbHealth - %s] [:info:] Primary changed from %s to %s", common.Config.Identifier, previous, primary)
		// Informational only: not an app-impacting condition by itself.
		common.Alarm(message, "", "", false)
	}

	_ = healthdb.PutJSON("mongodbHealth", "primary", primary, nil, time.Now())
}

// checkSecondaryQuorum evaluates whether enough healthy secondaries remain.
func checkSecondaryQuorum(members []MemberInfo) {
	healthy := 0
	for _, m := range members {
		if !m.IsPrimary && m.Healthy {
			healthy++
		}
	}

	minSecondaries := DbHealthConfig.Mongodb.Replicaset.Min_secondaries
	healthData.ReplicaSet.HealthySecondaries = healthy
	healthData.ReplicaSet.MinSecondaries = minSecondaries
	healthData.ReplicaSet.SecondaryQuorumOk = healthy >= minSecondaries

	if healthy < minSecondaries {
		msg := fmt.Sprintf("MongoDB healthy secondaries %d < required %d", healthy, minSecondaries)
		msgTr := fmt.Sprintf("MongoDB sağlıklı secondary sayısı %d < gerekli %d", healthy, minSecondaries)
		subject := fmt.Sprintf("%s için MongoDB secondary quorum sağlanamıyor", common.Config.Identifier)
		common.AlarmCheckDown("mongodb-secondary-quorum", msg, false, "", "")
		issues.CheckDown("mongodb-secondary-quorum", subject, msgTr, false, 0)
	} else {
		common.AlarmCheckUp("mongodb-secondary-quorum", fmt.Sprintf("Secondary quorum OK: %d/%d", healthy, minSecondaries), false)
		issues.CheckUp("mongodb-secondary-quorum", "MongoDB secondary quorum normale döndü")
	}

	// Individual unhealthy secondaries while quorum still holds: Zulip-only.
	for _, m := range members {
		if m.IsPrimary {
			continue
		}
		service := "mongodb-secondary-unhealthy-" + m.Name
		if !m.Healthy {
			common.AlarmCheckDown(service, fmt.Sprintf("MongoDB secondary %s is unhealthy (state=%s)", m.Name, m.StateStr), false, "", "")
		} else {
			common.AlarmCheckUp(service, fmt.Sprintf("MongoDB secondary %s is healthy", m.Name), false)
		}
	}
}

// checkReplicationLag evaluates the maximum secondary lag against warn/critical
// thresholds.
func checkReplicationLag(members []MemberInfo) {
	var maxLag float64
	for _, m := range members {
		if !m.IsPrimary && m.LagSeconds > maxLag {
			maxLag = m.LagSeconds
		}
	}

	warn := float64(DbHealthConfig.Mongodb.Replicaset.Lag_warn_seconds)
	critical := float64(DbHealthConfig.Mongodb.Replicaset.Lag_critical_seconds)

	healthData.ReplicaSet.MaxLagSeconds = maxLag
	healthData.ReplicaSet.LagWarnSeconds = warn
	healthData.ReplicaSet.LagCriticalSeconds = critical

	switch {
	case maxLag > critical:
		healthData.ReplicaSet.LagState = "critical"
		msg := fmt.Sprintf("MongoDB replication lag %.2fs > critical %.2fs", maxLag, critical)
		msgTr := fmt.Sprintf("MongoDB replikasyon gecikmesi %.2fs > kritik %.2fs", maxLag, critical)
		subject := fmt.Sprintf("%s için MongoDB replikasyon gecikmesi kritik seviyede", common.Config.Identifier)
		common.AlarmCheckDown("mongodb-lag-critical", msg, false, "", "")
		issues.CheckDown("mongodb-lag-critical", subject, msgTr, false, 0)
		common.AlarmCheckUp("mongodb-lag-warn", "Lag above critical, warn cleared", false)
	case maxLag > warn:
		healthData.ReplicaSet.LagState = "warn"
		msg := fmt.Sprintf("MongoDB replication lag %.2fs > warn %.2fs", maxLag, warn)
		// Zulip-only: within warn band, not yet app-impacting.
		common.AlarmCheckDown("mongodb-lag-warn", msg, false, "", "")
		common.AlarmCheckUp("mongodb-lag-critical", "Lag below critical", false)
		issues.CheckUp("mongodb-lag-critical", "MongoDB replikasyon gecikmesi kritik seviyenin altına döndü")
	default:
		healthData.ReplicaSet.LagState = "ok"
		common.AlarmCheckUp("mongodb-lag-warn", fmt.Sprintf("Lag OK: %.2fs", maxLag), false)
		common.AlarmCheckUp("mongodb-lag-critical", fmt.Sprintf("Lag OK: %.2fs", maxLag), false)
		issues.CheckUp("mongodb-lag-critical", "MongoDB replikasyon gecikmesi kritik seviyenin altına döndü")
	}
}

// checkOplogWindow computes the oplog time window (last - first entry) in
// hours and evaluates it against warn/critical thresholds.
func checkOplogWindow(ctx context.Context, client *mongo.Client) {
	window, err := oplogWindowHours(ctx, client)
	if err != nil {
		// Can't determine the window; skip without alarming (not all
		// deployments expose local.oplog.rs the same way).
		return
	}

	warn := DbHealthConfig.Mongodb.Replicaset.Oplog_window_warn_hours
	critical := DbHealthConfig.Mongodb.Replicaset.Oplog_window_critical_hours

	healthData.ReplicaSet.OplogWindowHours = window
	healthData.ReplicaSet.OplogWarnHours = warn
	healthData.ReplicaSet.OplogCriticalHours = critical

	switch {
	case window < critical:
		healthData.ReplicaSet.OplogState = "critical"
		msg := fmt.Sprintf("MongoDB oplog window %.2fh < critical %.2fh", window, critical)
		msgTr := fmt.Sprintf("MongoDB oplog penceresi %.2fs < kritik %.2fs", window, critical)
		subject := fmt.Sprintf("%s için MongoDB oplog penceresi kritik seviyede", common.Config.Identifier)
		common.AlarmCheckDown("mongodb-oplog-critical", msg, false, "", "")
		issues.CheckDown("mongodb-oplog-critical", subject, msgTr, false, 0)
		common.AlarmCheckUp("mongodb-oplog-warn", "Oplog window below critical, warn cleared", false)
	case window < warn:
		healthData.ReplicaSet.OplogState = "warn"
		msg := fmt.Sprintf("MongoDB oplog window %.2fh < warn %.2fh", window, warn)
		// Zulip-only: within warn band, not yet app-impacting.
		common.AlarmCheckDown("mongodb-oplog-warn", msg, false, "", "")
		common.AlarmCheckUp("mongodb-oplog-critical", "Oplog window above critical", false)
		issues.CheckUp("mongodb-oplog-critical", "MongoDB oplog penceresi kritik seviyenin üstüne döndü")
	default:
		healthData.ReplicaSet.OplogState = "ok"
		common.AlarmCheckUp("mongodb-oplog-warn", fmt.Sprintf("Oplog window OK: %.2fh", window), false)
		common.AlarmCheckUp("mongodb-oplog-critical", fmt.Sprintf("Oplog window OK: %.2fh", window), false)
		issues.CheckUp("mongodb-oplog-critical", "MongoDB oplog penceresi kritik seviyenin üstüne döndü")
	}
}

// oplogWindowHours computes (last - first) oplog entry timestamps, in hours,
// from local.oplog.rs using natural order for first and reverse natural
// order for last.
func oplogWindowHours(ctx context.Context, client *mongo.Client) (float64, error) {
	coll := client.Database("local").Collection("oplog.rs")

	var firstDoc, lastDoc bson.M

	err := coll.FindOne(ctx, bson.D{}, findOneOptionsNaturalAsc()).Decode(&firstDoc)
	if err != nil {
		return 0, err
	}
	err = coll.FindOne(ctx, bson.D{}, findOneOptionsNaturalDesc()).Decode(&lastDoc)
	if err != nil {
		return 0, err
	}

	firstTS, ok1 := firstDoc["ts"].(bson.Timestamp)
	lastTS, ok2 := lastDoc["ts"].(bson.Timestamp)
	if !ok1 || !ok2 {
		return 0, fmt.Errorf("unexpected oplog ts field type")
	}

	first := time.Unix(int64(firstTS.T), 0)
	last := time.Unix(int64(lastTS.T), 0)

	return last.Sub(first).Hours(), nil
}

// findOneOptionsNaturalAsc sorts by natural (insertion) order ascending,
// i.e. the oldest oplog entry first.
func findOneOptionsNaturalAsc() *options.FindOneOptionsBuilder {
	return options.FindOne().SetSort(bson.D{{Key: "$natural", Value: 1}})
}

// findOneOptionsNaturalDesc sorts by natural (insertion) order descending,
// i.e. the newest oplog entry first.
func findOneOptionsNaturalDesc() *options.FindOneOptionsBuilder {
	return options.FindOne().SetSort(bson.D{{Key: "$natural", Value: -1}})
}

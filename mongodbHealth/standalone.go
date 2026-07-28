//go:build linux

package mongodbHealth

import (
	"context"
	"fmt"

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// serverStatusResult holds the subset of the `serverStatus` command output
// that mongodbHealth cares about.
type serverStatusResult struct {
	Connections struct {
		Current   int32 `bson:"current"`
		Available int32 `bson:"available"`
	} `bson:"connections"`
	WiredTiger struct {
		Cache struct {
			BytesInCache       int64 `bson:"bytes currently in the cache"`
			MaxBytesConfigured int64 `bson:"maximum bytes configured"`
		} `bson:"cache"`
		ConcurrentTransactions struct {
			Write struct {
				Available int32 `bson:"available"`
			} `bson:"write"`
			Read struct {
				Available int32 `bson:"available"`
			} `bson:"read"`
		} `bson:"concurrentTransactions"`
	} `bson:"wiredTiger"`
}

// CheckStandalone runs serverStatus and evaluates connection usage, WiredTiger
// cache usage and ticket exhaustion. Applies to any node (standalone or
// replica set member).
func CheckStandalone(ctx context.Context, client *mongo.Client) {
	var status serverStatusResult
	err := client.Database("admin").RunCommand(ctx, bson.D{{Key: "serverStatus", Value: 1}}).Decode(&status)
	if err != nil {
		if isAuthError(err) {
			// Connection itself is fine (we got a real server response), but
			// we lack permission to read serverStatus, so health cannot be
			// assessed at all. This must not be silently reported as
			// "everything is zero/ok" -> alarm clearly, regardless of
			// whether this node is meant to be standalone or a replica set
			// member.
			msg := fmt.Sprintf("Could not run serverStatus: insufficient permissions (%s)", err.Error())
			subject := fmt.Sprintf("%s için MongoDB metrikleri okunamıyor", common.Config.Identifier)
			msgTr := fmt.Sprintf("MongoDB serverStatus komutu yetki hatasından dolayı çalıştırılamadı: %s", err.Error())
			appendPermissionWarning(msg)
			common.AlarmCheckDown("mongodb-metrics-permission", msg, false, "", "")
			issues.CheckDown("mongodb-metrics-permission", subject, msgTr, false, 0)
			common.AlarmCheckUp("mongodb-connection", "MongoDB connection is up (metrics unavailable)", false)
			return
		}
		msg := fmt.Sprintf("Couldn't run serverStatus: %s", err.Error())
		common.AlarmCheckDown("mongodb-connection", msg, false, "", "")
		healthData.Standalone.ConnectionsExceeded = false
		return
	}
	common.AlarmCheckUp("mongodb-connection", "serverStatus OK", false)
	common.AlarmCheckUp("mongodb-metrics-permission", "serverStatus permissions OK", false)
	issues.CheckUp("mongodb-metrics-permission", "MongoDB metrikleri tekrar okunabiliyor")

	checkConnections(status)
	checkCacheUsage(status)
	checkTicketExhaustion(status)
}

func checkConnections(status serverStatusResult) {
	current := int(status.Connections.Current)
	available := int(status.Connections.Available)
	total := current + available

	healthData.Standalone.ConnectionsCurrent = current
	healthData.Standalone.ConnectionsAvailable = available

	limit := float64(DbHealthConfig.Mongodb.Limits.Connections_percent)
	healthData.Standalone.ConnectionsLimit = limit

	var percent float64
	if total > 0 {
		percent = float64(current) / float64(total) * 100
	}
	healthData.Standalone.ConnectionsPercent = percent
	healthData.Standalone.ConnectionsExceeded = percent > limit

	if percent > limit {
		msg := fmt.Sprintf("MongoDB connections usage %.2f%% > %.2f%% (%d/%d)", percent, limit, current, total)
		// Zulip-only: connections nearing the limit is a warning signal, not
		// necessarily app-impacting yet.
		common.AlarmCheckDown("mongodb-connections-limit", msg, false, "", "")
	} else {
		common.AlarmCheckUp("mongodb-connections-limit", fmt.Sprintf("Connections usage OK: %.2f%%/%.2f%%", percent, limit), false)
	}
}

func checkCacheUsage(status serverStatusResult) {
	used := status.WiredTiger.Cache.BytesInCache
	maxBytes := status.WiredTiger.Cache.MaxBytesConfigured

	limit := float64(DbHealthConfig.Mongodb.Limits.Cache_percent)
	healthData.Standalone.CacheLimit = limit

	var percent float64
	if maxBytes > 0 {
		percent = float64(used) / float64(maxBytes) * 100
	}
	healthData.Standalone.CacheUsedPercent = percent
	healthData.Standalone.CacheExceeded = percent > limit

	if percent > limit {
		msg := fmt.Sprintf("MongoDB WiredTiger cache usage %.2f%% > %.2f%%", percent, limit)
		// Zulip-only: elevated cache pressure, not necessarily app-impacting.
		common.AlarmCheckDown("mongodb-cache-limit", msg, false, "", "")
	} else {
		common.AlarmCheckUp("mongodb-cache-limit", fmt.Sprintf("Cache usage OK: %.2f%%/%.2f%%", percent, limit), false)
	}
}

func checkTicketExhaustion(status serverStatusResult) {
	readAvail := int(status.WiredTiger.ConcurrentTransactions.Read.Available)
	writeAvail := int(status.WiredTiger.ConcurrentTransactions.Write.Available)

	healthData.Standalone.TicketsAvailableRead = readAvail
	healthData.Standalone.TicketsAvailableWrite = writeAvail

	exhausted := readAvail <= 0 || writeAvail <= 0
	healthData.Standalone.TicketsExhausted = exhausted

	if exhausted {
		msg := fmt.Sprintf("MongoDB WiredTiger tickets exhausted: read available=%d write available=%d", readAvail, writeAvail)
		msgTr := fmt.Sprintf("MongoDB WiredTiger bilet tükenmesi: okuma müsait=%d yazma müsait=%d", readAvail, writeAvail)
		subject := fmt.Sprintf("%s için MongoDB WiredTiger bilet tükenmesi", common.Config.Identifier)
		// Zulip+Redmine: ticket exhaustion blocks reads/writes -> app-impacting.
		common.AlarmCheckDown("mongodb-ticket-exhaustion", msg, false, "", "")
		issues.CheckDown("mongodb-ticket-exhaustion", subject, msgTr, false, 0)
	} else {
		common.AlarmCheckUp("mongodb-ticket-exhaustion", fmt.Sprintf("Tickets OK: read=%d write=%d", readAvail, writeAvail), false)
		issues.CheckUp("mongodb-ticket-exhaustion", "MongoDB WiredTiger biletleri normale döndü")
	}
}

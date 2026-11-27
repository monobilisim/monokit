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
		// ConcurrentTransactions is the legacy (pre-7.x) ticket location. On
		// MongoDB 7.0+/8.0 with the "Execution Control" rework, this section
		// is absent from serverStatus and Queues (below) must be used
		// instead. TotalTickets is captured so we can detect which source is
		// actually populated for this server version.
		ConcurrentTransactions struct {
			Write struct {
				Available    int32 `bson:"available"`
				TotalTickets int32 `bson:"totalTickets"`
			} `bson:"write"`
			Read struct {
				Available    int32 `bson:"available"`
				TotalTickets int32 `bson:"totalTickets"`
			} `bson:"read"`
		} `bson:"concurrentTransactions"`
	} `bson:"wiredTiger"`
	// Queues is the MongoDB 7.0+/8.0 "Execution Control" replacement for
	// wiredTiger.concurrentTransactions.
	Queues struct {
		Execution struct {
			Write executionQueue `bson:"write"`
			Read  executionQueue `bson:"read"`
		} `bson:"execution"`
	} `bson:"queues"`
}

// executionQueue is one direction (read/write) of queues.execution.
type executionQueue struct {
	Available      int32 `bson:"available"`
	TotalTickets   int32 `bson:"totalTickets"`
	NormalPriority struct {
		QueueLength int64 `bson:"queueLength"`
	} `bson:"normalPriority"`
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
			appendPermissionWarning(msg)
			// Zulip only - no Redmine issue for metrics-permission per user request.
			common.AlarmCheckDown("mongodb-metrics-permission", msg, false, "", "")
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

// ticketPressure decides whether WiredTiger tickets are saturated.
//
// Before MongoDB 7.0 the ticket pool is fixed (128), so no available ticket
// means operations are blocked. From 7.0 on, throughputProbing resizes the
// pool on its own (seen between 5 and 128), so available=0 is routine and
// says nothing about blocking. There the operations waiting for a ticket
// (normalPriority.queueLength) are used instead.
func ticketPressure(status serverStatusResult, queueLimit int64) (saturated bool, msg string) {
	legacy := status.WiredTiger.ConcurrentTransactions
	if legacy.Read.TotalTickets > 0 || legacy.Write.TotalTickets > 0 {
		if legacy.Read.Available <= 0 || legacy.Write.Available <= 0 {
			return true, fmt.Sprintf("MongoDB WiredTiger tickets exhausted: read available=%d write available=%d", legacy.Read.Available, legacy.Write.Available)
		}
		return false, fmt.Sprintf("Tickets OK: read=%d write=%d", legacy.Read.Available, legacy.Write.Available)
	}
	readQ := status.Queues.Execution.Read.NormalPriority.QueueLength
	writeQ := status.Queues.Execution.Write.NormalPriority.QueueLength
	if readQ >= queueLimit || writeQ >= queueLimit {
		return true, fmt.Sprintf("MongoDB WiredTiger ticket queue >= %d: read queue=%d write queue=%d", queueLimit, readQ, writeQ)
	}
	return false, fmt.Sprintf("Ticket queue OK: read=%d write=%d", readQ, writeQ)
}

func checkTicketExhaustion(status serverStatusResult) {
	queueLimit := int64(DbHealthConfig.Mongodb.Limits.Ticket_queue_length)
	if queueLimit <= 0 {
		queueLimit = 50
	}
	minutes := DbHealthConfig.Mongodb.Limits.Ticket_queue_minutes
	if minutes <= 0 {
		minutes = 5
	}

	readAvail := int(status.WiredTiger.ConcurrentTransactions.Read.Available)
	writeAvail := int(status.WiredTiger.ConcurrentTransactions.Write.Available)
	if status.WiredTiger.ConcurrentTransactions.Read.TotalTickets == 0 &&
		status.WiredTiger.ConcurrentTransactions.Write.TotalTickets == 0 {
		readAvail = int(status.Queues.Execution.Read.Available)
		writeAvail = int(status.Queues.Execution.Write.Available)
	}
	healthData.Standalone.TicketsAvailableRead = readAvail
	healthData.Standalone.TicketsAvailableWrite = writeAvail
	healthData.Standalone.TicketQueueRead = status.Queues.Execution.Read.NormalPriority.QueueLength
	healthData.Standalone.TicketQueueWrite = status.Queues.Execution.Write.NormalPriority.QueueLength

	saturated, msg := ticketPressure(status, queueLimit)
	healthData.Standalone.TicketsExhausted = saturated

	// Zulip-only, and only once the pressure lasts `minutes`: short queue
	// spikes during batch jobs resolve on their own.
	if saturated {
		common.AlarmCheckDownWithInterval("mongodb-ticket-exhaustion", msg, false, "", "", minutes)
	} else {
		common.AlarmCheckUpWithInterval("mongodb-ticket-exhaustion", msg, false, recoveryMinutes)
		// Closes issues opened by earlier versions, which raised one per event.
		issues.CheckUp("mongodb-ticket-exhaustion", "MongoDB WiredTiger biletleri normale döndü")
	}
}

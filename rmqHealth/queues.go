//go:build linux

package rmqHealth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/rs/zerolog/log"
)

// queueAPIResponse represents a queue entry from the RabbitMQ HTTP API.
// The rabbithole struct does not include slave_nodes/synchronised_slave_nodes fields,
// so this struct is populated directly from the HTTP API response.
type queueAPIResponse struct {
	Name                   string `json:"name"`
	Vhost                  string `json:"vhost"`
	State                  string `json:"state"`
	Type                   string `json:"type"`
	Durable                bool   `json:"durable"`
	Node                   string `json:"node"`
	Messages               int    `json:"messages"`
	MessagesReady          int    `json:"messages_ready"`
	MessagesUnacknowledged int    `json:"messages_unacknowledged"`
	Consumers              int    `json:"consumers"`
	Policy                 string `json:"policy"`
	// Classic mirrored queue fields
	SlaveNodes             []string `json:"slave_nodes"`
	SynchronisedSlaveNodes []string `json:"synchronised_slave_nodes"`
	// Quorum queue fields
	Members []string `json:"members"`
	Online  []string `json:"online"`
	Leader  string   `json:"leader"`
}

// fetchQueuesFromAPI fetches all queues from the RabbitMQ HTTP API.
func fetchQueuesFromAPI() ([]queueAPIResponse, error) {
	url := "http://localhost:15672/api/queues"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.SetBasicAuth(Config.User, Config.Password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var queues []queueAPIResponse
	if err := json.Unmarshal(body, &queues); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return queues, nil
}

// isIgnored reports whether a queue name is in the ignore list.
func isIgnored(name string) bool {
	return slices.Contains(Config.Queues.IgnoreQueues, name)
}

// checkQueues checks the health of all queues and fires alarms as needed.
func checkQueues() {
	queues, err := fetchQueuesFromAPI()
	if err != nil {
		log.Error().Err(err).Str("component", "rmqHealth").Str("operation", "checkQueues").Msg("Failed to fetch queue list")
		common.AlarmCheckDown("rabbitmq_queues_api", "Failed to fetch queue list: "+err.Error(), false, "", "")
		healthData.Queues.FetchOK = false
		return
	}
	common.AlarmCheckUp("rabbitmq_queues_api", "RabbitMQ queue list is now reachable", false)
	healthData.Queues.FetchOK = true

	// Determine expected mirror count — only when MirrorSyncCheck is enabled
	expectedMirrors := 0
	if Config.Queues.MirrorSyncCheck {
		expectedMirrors = Config.Queues.ExpectedMirrorCount
		if expectedMirrors == 0 {
			clusterNodeCount := len(healthData.Cluster.Nodes)
			if clusterNodeCount > 1 {
				expectedMirrors = clusterNodeCount - 1
			}
		}
	}

	healthData.Queues.Items = []QueueHealthItem{}
	stoppedCount := 0
	noConsumerCount := 0
	highMessageCount := 0
	collector := &queueProblemCollector{}

	for _, q := range queues {
		if isIgnored(q.Name) {
			log.Debug().Str("queue", q.Name).Msg("Queue is in ignore list, skipping")
			continue
		}

		item := QueueHealthItem{
			Name:                   q.Name,
			State:                  q.State,
			Type:                   q.Type,
			Messages:               q.Messages,
			MessagesUnacknowledged: q.MessagesUnacknowledged,
			Consumers:              q.Consumers,
			Node:                   q.Node,
		}

		// --- 1. Stopped queue check ---
		if q.State != "running" {
			stoppedCount++
			item.Stopped = true
			alarmKey := "rabbitmq_queue_stopped_" + sanitizeAlarmKey(q.Name)
			collector.addAlarm(alarmKey, fmt.Sprintf("Queue `%s` is in state: **%s** (node: %s)", q.Name, q.State, q.Node))
			collector.addProblem(problemStopped, q.Name,
				fmt.Sprintf("durum: %s, node: %s", q.State, q.Node))
		} else {
			item.Stopped = false
			alarmKey := "rabbitmq_queue_stopped_" + sanitizeAlarmKey(q.Name)
			common.AlarmCheckUp(alarmKey, fmt.Sprintf("Queue `%s` is now running", q.Name), false)
			resolveRedmineIssue(alarmKey, q.Name,
				common.Config.Identifier+" için `"+q.Name+"` kuyruğu tekrar çalışır durumda")
		}

		// --- 2. Consumer check ---
		if Config.Queues.ConsumerCheck && q.Consumers == 0 {
			noConsumerCount++
			item.NoConsumer = true
			alarmKey := "rabbitmq_queue_no_consumer_" + sanitizeAlarmKey(q.Name)
			msg := fmt.Sprintf("Queue `%s` has no active consumers (messages: %d)", q.Name, q.Messages)
			common.AlarmCheckDown(alarmKey, msg, false, "", "")
		} else if Config.Queues.ConsumerCheck {
			item.NoConsumer = false
			alarmKey := "rabbitmq_queue_no_consumer_" + sanitizeAlarmKey(q.Name)
			common.AlarmCheckUp(alarmKey, fmt.Sprintf("Queue `%s` now has active consumers (%d)", q.Name, q.Consumers), false)
		}

		// --- 3. Message backlog check ---
		if Config.Queues.MessageThreshold > 0 && q.Messages > Config.Queues.MessageThreshold {
			highMessageCount++
			item.HighMessages = true
			alarmKey := "rabbitmq_queue_high_messages_" + sanitizeAlarmKey(q.Name)
			msg := fmt.Sprintf("Queue `%s` message count exceeded threshold: %d > %d (unacked: %d)",
				q.Name, q.Messages, Config.Queues.MessageThreshold, q.MessagesUnacknowledged)
			common.AlarmCheckDown(alarmKey, msg, false, "", "")
		} else if Config.Queues.MessageThreshold > 0 {
			item.HighMessages = false
			alarmKey := "rabbitmq_queue_high_messages_" + sanitizeAlarmKey(q.Name)
			common.AlarmCheckUp(alarmKey, fmt.Sprintf("Queue `%s` message count is back to normal: %d", q.Name, q.Messages), false)
		}

		// --- 4. Mirror/Sync kontrolü ---
		if Config.Queues.MirrorSyncCheck && expectedMirrors > 0 {
			item.SyncStatus = checkQueueMirrorSync(q, expectedMirrors, collector)
		}

		healthData.Queues.Items = append(healthData.Queues.Items, item)
	}

	healthData.Queues.TotalCount = len(healthData.Queues.Items)
	healthData.Queues.StoppedCount = stoppedCount
	healthData.Queues.NoConsumerCount = noConsumerCount
	healthData.Queues.HighMessageCount = highMessageCount
	healthData.Queues.UnsyncedCount = collector.unsynced

	if stoppedCount > 0 || collector.unsynced > 0 {
		healthData.IsHealthy = false
	}

	// All failing queues share one Redmine issue; the alarms are sent afterwards
	// so each one can point at it.
	issueLink := syncQueueRedmineIssue(collector.problems)
	for _, a := range collector.alarms {
		msg := a.msg
		if issueLink != "" {
			msg = msg + "\n\n" + issueLink
		}
		common.AlarmCheckDown(a.key, msg, false, "", "")
	}

	log.Debug().
		Int("total", healthData.Queues.TotalCount).
		Int("stopped", stoppedCount).
		Int("no_consumer", noConsumerCount).
		Int("high_messages", highMessageCount).
		Int("unsynced", collector.unsynced).
		Msg("Queue health check completed")
}

// checkQueueMirrorSync checks the mirror/replica sync status of a queue.
// Uses different fields depending on queue type:
//   - classic: slave_nodes / synchronised_slave_nodes
//   - quorum:  members / online
func checkQueueMirrorSync(q queueAPIResponse, expectedMirrors int, collector *queueProblemCollector) QueueSyncStatus {
	status := QueueSyncStatus{}

	switch q.Type {
	case "quorum":
		totalMembers := len(q.Members)
		onlineMembers := len(q.Online)
		status.TotalReplicas = totalMembers
		status.SyncedReplicas = onlineMembers
		status.IsFullySynced = onlineMembers >= (expectedMirrors + 1)

		alarmKey := "rabbitmq_queue_sync_" + sanitizeAlarmKey(q.Name)
		if !status.IsFullySynced {
			collector.unsynced++
			missingNodes := findMissingNodes(q.Members, q.Online)
			collector.addAlarm(alarmKey, fmt.Sprintf("Quorum queue `%s` has insufficient online members (%d/%d); missing: %s",
				q.Name, onlineMembers, totalMembers, strings.Join(missingNodes, ", ")))
			collector.addProblem(problemUnsynced, q.Name,
				fmt.Sprintf("tip: quorum, online: %d/%d, eksik: %s",
					onlineMembers, totalMembers, strings.Join(missingNodes, ", ")))
		} else {
			common.AlarmCheckUp(alarmKey, fmt.Sprintf("Quorum queue `%s` is online on all members (%d/%d)",
				q.Name, onlineMembers, totalMembers), false)
			resolveRedmineIssue(alarmKey, q.Name,
				common.Config.Identifier+" için `"+q.Name+"` kuyruğu tüm üyeleriyle senkronize")
		}

	default:
		syncedCount := len(q.SynchronisedSlaveNodes)
		slaveCount := len(q.SlaveNodes)
		status.TotalReplicas = slaveCount + 1
		status.SyncedReplicas = syncedCount
		status.IsFullySynced = syncedCount >= expectedMirrors

		alarmKey := "rabbitmq_queue_sync_" + sanitizeAlarmKey(q.Name)
		if !status.IsFullySynced {
			collector.unsynced++
			unsyncedSlaves := findMissingNodes(q.SlaveNodes, q.SynchronisedSlaveNodes)
			collector.addAlarm(alarmKey, fmt.Sprintf("Classic queue `%s` has insufficient synchronised mirrors (%d/%d, expected %d); not synced: %s",
				q.Name, syncedCount, slaveCount, expectedMirrors, strings.Join(unsyncedSlaves, ", ")))
			collector.addProblem(problemUnsynced, q.Name,
				fmt.Sprintf("tip: classic, sync: %d/%d (beklenen: %d), sync olmayan: %s",
					syncedCount, slaveCount, expectedMirrors, strings.Join(unsyncedSlaves, ", ")))
		} else {
			common.AlarmCheckUp(alarmKey, fmt.Sprintf("Classic queue `%s` is fully synchronised across all mirrors (%d/%d)",
				q.Name, syncedCount, expectedMirrors), false)
			resolveRedmineIssue(alarmKey, q.Name,
				common.Config.Identifier+" için `"+q.Name+"` kuyruğu tüm mirror'larla senkronize")
		}
	}

	return status
}

// findMissingNodes returns nodes that are in all but not in synced.
func findMissingNodes(all []string, synced []string) []string {
	syncedSet := make(map[string]struct{}, len(synced))
	for _, n := range synced {
		syncedSet[n] = struct{}{}
	}
	var missing []string
	for _, n := range all {
		if _, ok := syncedSet[n]; !ok {
			missing = append(missing, n)
		}
	}
	return missing
}

func sanitizeAlarmKey(name string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		".", "_",
		" ", "_",
	)
	return replacer.Replace(name)
}

func isRedmineExcluded(name string) bool {
	return slices.Contains(Config.Queues.Redmine.ExcludeQueues, name)
}

// resolveRedmineIssue closes the per-queue issue an older monokit version may
// have opened for this queue. Failing queues now share one issue (see
// syncQueueRedmineIssue), but issues opened before the upgrade still have to be
// closed when their queue recovers; for every other queue this is a no-op.
func resolveRedmineIssue(alarmKey, queueName, message string) {
	if !Config.Queues.Redmine.Enabled || isRedmineExcluded(queueName) {
		return
	}
	issues.CheckUp(alarmKey, message)
}

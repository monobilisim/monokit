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
	"github.com/rs/zerolog/log"
)

// queueAPIResponse, RabbitMQ HTTP API'sinden gelen kuyruk verisini temsil eder.
// rabbithole struct'ı slave_nodes/synchronised_slave_nodes alanlarını içermediğinden
// bu struct HTTP API'den direkt alınır.
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
	// Classic mirrored queue alanları
	SlaveNodes             []string `json:"slave_nodes"`
	SynchronisedSlaveNodes []string `json:"synchronised_slave_nodes"`
	// Quorum queue alanları
	Members []string `json:"members"`
	Online  []string `json:"online"`
	Leader  string   `json:"leader"`
}

// fetchQueuesFromAPI, RabbitMQ HTTP API'sinden tüm kuyrukları çeker.
func fetchQueuesFromAPI() ([]queueAPIResponse, error) {
	url := "http://localhost:15672/api/queues"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("HTTP request oluşturulamadı: %w", err)
	}
	req.SetBasicAuth(Config.User, Config.Password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d döndü", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("response body okunamadı: %w", err)
	}

	var queues []queueAPIResponse
	if err := json.Unmarshal(body, &queues); err != nil {
		return nil, fmt.Errorf("JSON parse hatası: %w", err)
	}

	return queues, nil
}

// isIgnored, bir kuyruğun ignore listesinde olup olmadığını kontrol eder.
func isIgnored(name string) bool {
	return slices.Contains(Config.Queues.IgnoreQueues, name)
}

// checkQueues, tüm kuyrukların sağlık durumunu kontrol eder ve alarm üretir.
func checkQueues() {
	queues, err := fetchQueuesFromAPI()
	if err != nil {
		log.Error().Err(err).Str("component", "rmqHealth").Str("operation", "checkQueues").Msg("Kuyruk listesi alınamadı")
		common.AlarmCheckDown("rabbitmq_queues_api", "Kuyruk listesi alınamadı: "+err.Error(), false, "", "")
		healthData.Queues.FetchOK = false
		return
	}
	common.AlarmCheckUp("rabbitmq_queues_api", "Kuyruk listesi başarıyla alındı", false)
	healthData.Queues.FetchOK = true

	// Cluster node sayısını belirle (mirror beklentisi için) — sadece MirrorSyncCheck aktifse hesapla
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
	unsyncedCount := 0

	for _, q := range queues {
		if isIgnored(q.Name) {
			log.Debug().Str("queue", q.Name).Msg("Kuyruk ignore listesinde, atlanıyor")
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

		// --- 1. Stopped kuyruk kontrolü ---
		if q.State != "running" {
			stoppedCount++
			item.Stopped = true
			alarmKey := "rabbitmq_queue_stopped_" + sanitizeAlarmKey(q.Name)
			msg := fmt.Sprintf("Kuyruk `%s` durumu: **%s** (node: %s)", q.Name, q.State, q.Node)
			common.AlarmCheckDown(alarmKey, msg, false, "", "")
		} else {
			item.Stopped = false
			alarmKey := "rabbitmq_queue_stopped_" + sanitizeAlarmKey(q.Name)
			common.AlarmCheckUp(alarmKey, fmt.Sprintf("Kuyruk `%s` artık running durumunda", q.Name), false)
		}

		// --- 2. Consumer kontrolü ---
		if Config.Queues.ConsumerCheck && q.Consumers == 0 {
			noConsumerCount++
			item.NoConsumer = true
			alarmKey := "rabbitmq_queue_no_consumer_" + sanitizeAlarmKey(q.Name)
			msg := fmt.Sprintf("Kuyruk `%s` için aktif consumer yok (mesaj: %d)", q.Name, q.Messages)
			common.AlarmCheckDown(alarmKey, msg, false, "", "")
		} else if Config.Queues.ConsumerCheck {
			item.NoConsumer = false
			alarmKey := "rabbitmq_queue_no_consumer_" + sanitizeAlarmKey(q.Name)
			common.AlarmCheckUp(alarmKey, fmt.Sprintf("Kuyruk `%s` için consumer mevcut (%d)", q.Name, q.Consumers), false)
		}

		// --- 3. Mesaj birikimi kontrolü ---
		if Config.Queues.MessageThreshold > 0 && q.Messages > Config.Queues.MessageThreshold {
			highMessageCount++
			item.HighMessages = true
			alarmKey := "rabbitmq_queue_high_messages_" + sanitizeAlarmKey(q.Name)
			msg := fmt.Sprintf("Kuyruk `%s` mesaj sayısı eşiği aştı: %d > %d (unacked: %d)",
				q.Name, q.Messages, Config.Queues.MessageThreshold, q.MessagesUnacknowledged)
			common.AlarmCheckDown(alarmKey, msg, false, "", "")
		} else if Config.Queues.MessageThreshold > 0 {
			item.HighMessages = false
			alarmKey := "rabbitmq_queue_high_messages_" + sanitizeAlarmKey(q.Name)
			common.AlarmCheckUp(alarmKey, fmt.Sprintf("Kuyruk `%s` mesaj sayısı normal: %d", q.Name, q.Messages), false)
		}

		// --- 4. Mirror/Sync kontrolü ---
		if Config.Queues.MirrorSyncCheck && expectedMirrors > 0 {
			item.SyncStatus = checkQueueMirrorSync(q, expectedMirrors, &unsyncedCount)
		}

		healthData.Queues.Items = append(healthData.Queues.Items, item)
	}

	healthData.Queues.TotalCount = len(healthData.Queues.Items)
	healthData.Queues.StoppedCount = stoppedCount
	healthData.Queues.NoConsumerCount = noConsumerCount
	healthData.Queues.HighMessageCount = highMessageCount
	healthData.Queues.UnsyncedCount = unsyncedCount

	if stoppedCount > 0 || unsyncedCount > 0 {
		healthData.IsHealthy = false
	}

	log.Debug().
		Int("total", healthData.Queues.TotalCount).
		Int("stopped", stoppedCount).
		Int("no_consumer", noConsumerCount).
		Int("high_messages", highMessageCount).
		Int("unsynced", unsyncedCount).
		Msg("Kuyruk kontrolü tamamlandı")
}

// checkQueueMirrorSync, bir kuyruğun mirror senkronizasyonunu kontrol eder.
// Kuyruk tiplerine göre farklı alanları kullanır:
//   - classic: slave_nodes / synchronised_slave_nodes
//   - quorum: members / online
func checkQueueMirrorSync(q queueAPIResponse, expectedMirrors int, unsyncedCount *int) QueueSyncStatus {
	status := QueueSyncStatus{}

	switch q.Type {
	case "quorum":
		// Quorum queue: online member sayısı beklenen node sayısına eşit olmalı
		totalMembers := len(q.Members)
		onlineMembers := len(q.Online)
		status.TotalReplicas = totalMembers
		status.SyncedReplicas = onlineMembers
		status.IsFullySynced = onlineMembers >= (expectedMirrors + 1) // +1 master dahil

		if !status.IsFullySynced {
			*unsyncedCount++
			alarmKey := "rabbitmq_queue_sync_" + sanitizeAlarmKey(q.Name)
			missingNodes := findMissingNodes(q.Members, q.Online)
			msg := fmt.Sprintf("Quorum kuyruk `%s`: online üye sayısı yetersiz (%d/%d). Eksik: %s",
				q.Name, onlineMembers, totalMembers, strings.Join(missingNodes, ", "))
			common.AlarmCheckDown(alarmKey, msg, false, "", "")
		} else {
			alarmKey := "rabbitmq_queue_sync_" + sanitizeAlarmKey(q.Name)
			common.AlarmCheckUp(alarmKey, fmt.Sprintf("Quorum kuyruk `%s` tüm üyeleriyle online (%d/%d)",
				q.Name, onlineMembers, totalMembers), false)
		}

	default:
		// Classic mirrored queue
		syncedCount := len(q.SynchronisedSlaveNodes)
		slaveCount := len(q.SlaveNodes)
		status.TotalReplicas = slaveCount + 1 // +1 master
		status.SyncedReplicas = syncedCount
		status.IsFullySynced = syncedCount >= expectedMirrors

		if !status.IsFullySynced {
			*unsyncedCount++
			alarmKey := "rabbitmq_queue_sync_" + sanitizeAlarmKey(q.Name)
			unsyncedSlaves := findMissingNodes(q.SlaveNodes, q.SynchronisedSlaveNodes)
			msg := fmt.Sprintf("Classic kuyruk `%s`: sync slave sayısı yetersiz (%d/%d beklenen %d). Sync olmayan: %s",
				q.Name, syncedCount, slaveCount, expectedMirrors, strings.Join(unsyncedSlaves, ", "))
			common.AlarmCheckDown(alarmKey, msg, false, "", "")
		} else {
			alarmKey := "rabbitmq_queue_sync_" + sanitizeAlarmKey(q.Name)
			common.AlarmCheckUp(alarmKey, fmt.Sprintf("Classic kuyruk `%s` tüm slave'lerle sync (%d/%d)",
				q.Name, syncedCount, expectedMirrors), false)
		}
	}

	return status
}

// findMissingNodes, all listesinde olup synced listesinde olmayan node'ları bulur.
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

// sanitizeAlarmKey, kuyruk adını alarm key'i için uygun formata getirir.
func sanitizeAlarmKey(name string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		".", "_",
		" ", "_",
	)
	return replacer.Replace(name)
}

//go:build linux

package rmqHealth

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/healthdb"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/rs/zerolog/log"
)

// Failing queues are reported through a single Redmine issue instead of one
// issue per queue: when RabbitMQ itself breaks, every queue starts failing at
// once and the per-queue behaviour buried Redmine under near-identical issues.
// The failing queues are collected during a run, listed in the issue body, and
// the list is updated with a note whenever the set of failing queues changes.
const (
	queueIssueService = "rabbitmq_queues"
	queueProblemsKey  = "queues:redmine:problems"

	problemStopped  = "stopped"
	problemUnsynced = "unsynced"
)

// queueProblem is a single failing queue as it appears in the aggregate issue.
type queueProblem struct {
	Kind   string // problemStopped or problemUnsynced
	Name   string
	Detail string
}

// id identifies the problem across runs; it deliberately leaves Detail out so
// that changing message/replica counts do not count as a list change.
func (p queueProblem) id() string { return p.Kind + "|" + p.Name }

// pendingAlarm is a down alarm held back until the aggregate issue is known,
// so its link can be appended to the message.
type pendingAlarm struct {
	key string
	msg string
}

// queueProblemCollector gathers everything the aggregate issue and the delayed
// alarms need while the queues are being walked.
type queueProblemCollector struct {
	problems []queueProblem
	alarms   []pendingAlarm
	unsynced int
}

func (c *queueProblemCollector) addProblem(kind, name, detail string) {
	if !Config.Queues.Redmine.Enabled || isRedmineExcluded(name) {
		return
	}
	c.problems = append(c.problems, queueProblem{Kind: kind, Name: name, Detail: detail})
}

func (c *queueProblemCollector) addAlarm(key, msg string) {
	c.alarms = append(c.alarms, pendingAlarm{key: key, msg: msg})
}

var problemLabels = map[string]string{
	problemStopped:  "Çalışmayan kuyruklar",
	problemUnsynced: "Senkronize olmayan kuyruklar",
}

var problemShortLabels = map[string]string{
	problemStopped:  "çalışmıyor",
	problemUnsynced: "senkronize değil",
}

// syncQueueRedmineIssue opens, updates or closes the single queue issue and
// returns a "Redmine Issue: <url>" line for the alarms, or "" if there is none.
func syncQueueRedmineIssue(problems []queueProblem) string {
	if !Config.Queues.Redmine.Enabled {
		return ""
	}

	sortProblems(problems)
	ids := problemIDs(problems)

	if len(problems) == 0 {
		issues.CheckUp(queueIssueService, common.Config.Identifier+" için RabbitMQ kuyruklarında hata kalmadı; tüm kuyruklar çalışıyor ve senkronize.")
		clearQueueProblemIDs()
		return ""
	}

	prev, hadPrev := loadQueueProblemIDs()

	if issues.Show(queueIssueService) == "" {
		// No open issue yet — CheckDown opens one (honouring the configured
		// Redmine interval) with the current queue list as the description.
		subject := common.Config.Identifier + " için RabbitMQ kuyruklarında hata mevcut"
		issues.CheckDown(queueIssueService, subject, queueIssueBody(problems), false, 0)
		link := queueIssueLink()
		if link != "" {
			saveQueueProblemIDs(ids)
		}
		return link
	}

	// The issue is already open — add a note only when the failing queue set
	// changed, so a persisting failure does not keep re-notifying.
	if !hadPrev || !slices.Equal(prev, ids) {
		issues.Update(queueIssueService, queueIssueUpdateNote(prev, hadPrev, problems), true)
		saveQueueProblemIDs(ids)
	}

	return queueIssueLink()
}

func queueIssueLink() string {
	id := issues.Show(queueIssueService)
	if id == "" {
		return ""
	}
	return "Redmine Issue: " + common.GetRedmineDisplayUrl() + "/issues/" + id
}

// queueIssueBody is the issue description: a header plus the failing queues
// grouped by problem kind.
func queueIssueBody(problems []queueProblem) string {
	var sb strings.Builder
	sb.WriteString("RabbitMQ kuyruklarında hata tespit edildi.\n\n")
	sb.WriteString("Sunucu: " + common.Config.Identifier + "\n")
	sb.WriteString(fmt.Sprintf("Hatalı kuyruk sayısı: %d\n\n", len(problems)))
	sb.WriteString(queueProblemList(problems))
	return strings.TrimRight(sb.String(), "\n")
}

// queueIssueUpdateNote reports what changed since the last run and repeats the
// current list, so the issue always ends with the up-to-date set of queues.
func queueIssueUpdateNote(prev []string, hadPrev bool, problems []queueProblem) string {
	var sb strings.Builder
	sb.WriteString("Hatalı kuyruk listesi güncellendi.\n\n")

	if hadPrev {
		current := problemIDs(problems)
		var added []string
		for _, id := range current {
			if !slices.Contains(prev, id) {
				added = append(added, id)
			}
		}
		var recovered []string
		for _, id := range prev {
			if !slices.Contains(current, id) {
				recovered = append(recovered, id)
			}
		}
		if len(added) > 0 {
			sb.WriteString("Yeni hata veren kuyruklar:\n")
			for _, id := range added {
				sb.WriteString("- " + describeProblemID(id) + "\n")
			}
			sb.WriteString("\n")
		}
		if len(recovered) > 0 {
			sb.WriteString("Düzelen kuyruklar:\n")
			for _, id := range recovered {
				sb.WriteString("- " + describeProblemID(id) + "\n")
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString(fmt.Sprintf("Güncel hatalı kuyruk sayısı: %d\n\n", len(problems)))
	sb.WriteString(queueProblemList(problems))
	return strings.TrimRight(sb.String(), "\n")
}

// queueProblemList renders the failing queues grouped under a heading per kind.
func queueProblemList(problems []queueProblem) string {
	var sb strings.Builder
	for _, kind := range []string{problemStopped, problemUnsynced} {
		var lines []string
		for _, p := range problems {
			if p.Kind == kind {
				lines = append(lines, "- "+p.Name+" — "+p.Detail)
			}
		}
		if len(lines) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("%s (%d):\n", problemLabels[kind], len(lines)))
		sb.WriteString(strings.Join(lines, "\n"))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// describeProblemID turns a stored "kind|queue" id back into a readable line.
func describeProblemID(id string) string {
	kind, name, found := strings.Cut(id, "|")
	if !found {
		return id
	}
	if label, ok := problemShortLabels[kind]; ok {
		return name + " (" + label + ")"
	}
	return name
}

func sortProblems(problems []queueProblem) {
	sort.Slice(problems, func(i, j int) bool {
		if problems[i].Kind != problems[j].Kind {
			return problems[i].Kind < problems[j].Kind
		}
		return problems[i].Name < problems[j].Name
	})
}

func problemIDs(problems []queueProblem) []string {
	ids := make([]string, 0, len(problems))
	for _, p := range problems {
		ids = append(ids, p.id())
	}
	return ids
}

func loadQueueProblemIDs() ([]string, bool) {
	raw, _, _, found, err := healthdb.GetJSON("rmqHealth", queueProblemsKey)
	if err != nil || !found || raw == "" {
		return nil, false
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		log.Debug().Err(err).Str("component", "rmqHealth").Msg("Failed to parse stored queue problem list")
		return nil, false
	}
	return ids, true
}

func saveQueueProblemIDs(ids []string) {
	data, err := json.Marshal(ids)
	if err != nil {
		return
	}
	if err := healthdb.PutJSON("rmqHealth", queueProblemsKey, string(data), nil, time.Now()); err != nil {
		log.Error().Err(err).Str("component", "rmqHealth").Msg("Failed to store queue problem list")
	}
}

func clearQueueProblemIDs() {
	_ = healthdb.Delete("rmqHealth", queueProblemsKey)
}

//go:build linux

package mongodbHealth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/healthdb"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "monokit-mongodbhealth")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_STATE_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type recorder struct {
	mu   sync.Mutex
	msgs []string
}

func (r *recorder) handler(w http.ResponseWriter, req *http.Request) {
	var body struct{ Text string }
	_ = json.NewDecoder(req.Body).Decode(&body)
	r.mu.Lock()
	r.msgs = append(r.msgs, body.Text)
	r.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (r *recorder) messages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.msgs...)
}

const (
	warnSvc = "test-lag-warn"
	critSvc = "test-lag-critical"
)

func setup(t *testing.T) *recorder {
	t.Helper()
	rec := &recorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	t.Cleanup(srv.Close)
	common.ScriptName = "mongodbHealth"
	common.Config.Identifier = "test-mongodb1"
	common.Config.Alarm.Enabled = true
	common.Config.Alarm.Interval = 3
	common.Config.Alarm.Repeat_hours = 0
	common.Config.Alarm.Webhook_urls = []string{srv.URL}
	common.Config.Alarm.SMTP.Enabled = false
	common.ClearAlarmState(warnSvc)
	common.ClearAlarmState(critSvc)
	return rec
}

// age moves every stored alarm timestamp back by d to simulate time passing.
func age(d time.Duration) {
	shift := func(v string) string {
		p, err := time.Parse("2006-01-02 15:04:05 -0700", v)
		if v == "" || err != nil {
			return v
		}
		return p.Add(-d).Format("2006-01-02 15:04:05 -0700")
	}
	for _, s := range []string{warnSvc, critSvc} {
		raw, _, _, found, err := healthdb.GetJSON("alarm", s)
		if err != nil || !found {
			continue
		}
		var j common.ServiceFile
		_ = json.Unmarshal([]byte(raw), &j)
		j.Date = shift(j.Date)
		j.RecoveryDate = shift(j.RecoveryDate)
		data, _ := json.Marshal(j)
		_ = healthdb.PutJSON("alarm", s, string(data), nil, time.Now())
	}
}

func level(l string) {
	reportLevel(l, warnSvc, critSvc, "lag > warn", "lag > critical", "Lag OK")
}

func assertMessages(t *testing.T, rec *recorder, want ...string) {
	t.Helper()
	got := rec.messages()
	if len(got) != len(want) {
		t.Fatalf("got %d messages %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range want {
		if !strings.Contains(got[i], want[i]) {
			t.Fatalf("message %d = %q, want it to contain %q", i, got[i], want[i])
		}
	}
}

// The 06:00-08:00 pattern on hepiyi main: warn -> critical -> back and forth
// -> ok. The old code posted every transition ("warn cleared", "below
// critical", ...); now only warn, critical and the final recovery are sent.
func TestReportLevelEscalationAndBackPostsOncePerLevel(t *testing.T) {
	rec := setup(t)

	level("warn")
	age(4 * time.Minute)
	level("warn")
	level("critical")
	age(4 * time.Minute)
	level("critical")
	for range 3 {
		level("warn")
		age(2 * time.Minute)
		level("critical")
		age(2 * time.Minute)
	}
	level("warn")
	level("ok")
	age(5 * time.Minute)
	level("warn") // relapse cancels the pending recovery
	level("ok")
	age(10 * time.Minute)
	level("ok") // only 10 minutes since recovery restarted
	age(6 * time.Minute)
	level("ok")
	level("ok")

	assertMessages(t, rec, "lag > warn", "lag > critical", "Lag OK")
}

// A value hovering around the warn threshold yields one alarm and one recovery.
func TestReportLevelFlappingAroundWarn(t *testing.T) {
	rec := setup(t)

	level("warn")
	age(4 * time.Minute)
	level("warn")
	for range 5 {
		level("ok")
		age(3 * time.Minute)
		level("warn")
		age(1 * time.Minute)
	}
	level("ok")
	age(16 * time.Minute)
	level("ok")

	assertMessages(t, rec, "lag > warn", "Lag OK")
}

// A critical spike shorter than the alarm interval is never announced, and it
// neither hides nor re-sends the warn alarm around it.
func TestReportLevelUndeliveredCriticalDoesNotHideWarn(t *testing.T) {
	rec := setup(t)

	level("critical")
	age(1 * time.Minute)
	level("warn")
	age(4 * time.Minute)
	level("warn")
	age(4 * time.Minute)
	level("critical") // short spike, shorter than the 3 minute interval
	age(1 * time.Minute)
	level("warn")
	age(4 * time.Minute)
	level("warn")

	assertMessages(t, rec, "lag > warn")
}

func TestIsClusterReporter(t *testing.T) {
	members := func(selfState string) []replSetMember {
		return []replSetMember{
			{Name: "a:27017", StateStr: "PRIMARY", Self: selfState == "PRIMARY"},
			{Name: "b:27017", StateStr: "SECONDARY", Self: selfState == "SECONDARY"},
		}
	}
	if !isClusterReporter(members("PRIMARY")) {
		t.Error("primary must report")
	}
	if isClusterReporter(members("SECONDARY")) {
		t.Error("secondary must not report")
	}
	if !isClusterReporter(members("")) {
		t.Error("without a self member the node must still report")
	}
}

func TestTicketPressure(t *testing.T) {
	var modern serverStatusResult
	modern.Queues.Execution.Read.Available = 0 // routine under throughputProbing
	modern.Queues.Execution.Read.TotalTickets = 8
	modern.Queues.Execution.Read.NormalPriority.QueueLength = 7
	if sat, msg := ticketPressure(modern, 50); sat {
		t.Fatalf("available=0 with a short queue must not alarm: %s", msg)
	}
	modern.Queues.Execution.Read.NormalPriority.QueueLength = 50
	if sat, _ := ticketPressure(modern, 50); !sat {
		t.Fatal("read queue at the limit must alarm")
	}
	modern.Queues.Execution.Read.NormalPriority.QueueLength = 0
	modern.Queues.Execution.Write.NormalPriority.QueueLength = 80
	if sat, _ := ticketPressure(modern, 50); !sat {
		t.Fatal("write queue over the limit must alarm")
	}

	var legacy serverStatusResult
	legacy.WiredTiger.ConcurrentTransactions.Read.TotalTickets = 128
	legacy.WiredTiger.ConcurrentTransactions.Write.TotalTickets = 128
	legacy.WiredTiger.ConcurrentTransactions.Read.Available = 0
	legacy.WiredTiger.ConcurrentTransactions.Write.Available = 100
	if sat, _ := ticketPressure(legacy, 50); !sat {
		t.Fatal("pre-7.0 fixed pool with no read tickets must alarm")
	}
	legacy.WiredTiger.ConcurrentTransactions.Read.Available = 3
	if sat, _ := ticketPressure(legacy, 50); sat {
		t.Fatal("pre-7.0 pool with tickets left must not alarm")
	}
}

package common

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/monobilisim/monokit/common/healthdb"
)

type webhookRecorder struct {
	mu   sync.Mutex
	msgs []string
	fail bool
}

func (r *webhookRecorder) handler(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var body struct{ Text string }
	_ = json.NewDecoder(req.Body).Decode(&body)
	r.msgs = append(r.msgs, body.Text)
	w.WriteHeader(http.StatusOK)
}

func (r *webhookRecorder) setFail(v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fail = v
}

func (r *webhookRecorder) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = nil
}

func (r *webhookRecorder) messages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string{}, r.msgs...)
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "monokit-state")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_STATE_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func setupAlarmTest(t *testing.T, rec *webhookRecorder) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	t.Cleanup(srv.Close)
	ScriptName = "vaultHealth"
	Config.Identifier = "dijitalkurye-prod-vault2"
	Config.Alarm.Enabled = true
	Config.Alarm.Interval = 3
	Config.Alarm.Webhook_urls = []string{srv.URL}
	Config.Alarm.SMTP.Enabled = false
	_ = healthdb.Delete("alarm", "vault_sealed")
}

func storedState() ServiceFile {
	raw, _, _, found, err := healthdb.GetJSON("alarm", "vault_sealed")
	if err != nil || !found {
		return ServiceFile{}
	}
	var j ServiceFile
	_ = json.Unmarshal([]byte(raw), &j)
	return j
}

func storeState(state ServiceFile) {
	data, _ := json.Marshal(state)
	_ = healthdb.PutJSON("alarm", "vault_sealed", string(data), nil, time.Now())
}

// elapsed rewrites the stored date to simulate time passing without sleeping.
func elapsed(d time.Duration) {
	state := storedState()
	state.Date = time.Now().Add(-d).Format("2006-01-02 15:04:05 -0700")
	storeState(state)
}

func TestAlarmDownDeliversOnceAndLocks(t *testing.T) {
	rec := &webhookRecorder{}
	setupAlarmTest(t, rec)

	AlarmCheckDown("vault_sealed", "Vault is sealed", false, "", "")
	elapsed(4 * time.Minute)
	AlarmCheckDown("vault_sealed", "Vault is sealed", false, "", "")
	elapsed(4 * time.Minute)
	AlarmCheckDown("vault_sealed", "Vault is sealed", false, "", "")
	AlarmCheckUp("vault_sealed", "Vault is unsealed", false)

	msgs := rec.messages()
	if len(msgs) != 2 {
		t.Fatalf("expected sealed + unsealed, got %d: %v", len(msgs), msgs)
	}
}

func TestAlarmDownRetriesAfterFailedDelivery(t *testing.T) {
	rec := &webhookRecorder{}
	setupAlarmTest(t, rec)

	rec.setFail(true)
	AlarmCheckDown("vault_sealed", "Vault is sealed", false, "", "")
	elapsed(4 * time.Minute)
	AlarmCheckDown("vault_sealed", "Vault is sealed", false, "", "")
	if storedState().Locked {
		t.Fatal("state locked although no channel accepted the alarm")
	}

	rec.setFail(false)
	elapsed(4 * time.Minute)
	AlarmCheckDown("vault_sealed", "Vault is sealed", false, "", "")
	AlarmCheckUp("vault_sealed", "Vault is unsealed", false)

	msgs := rec.messages()
	if len(msgs) != 2 {
		t.Fatalf("expected sealed retry + unsealed, got %d: %v", len(msgs), msgs)
	}
}

func TestAlarmLockedRepeatsDaily(t *testing.T) {
	rec := &webhookRecorder{}
	setupAlarmTest(t, rec)

	AlarmCheckDown("vault_sealed", "Vault is sealed", false, "", "")
	elapsed(4 * time.Minute)
	AlarmCheckDown("vault_sealed", "Vault is sealed", false, "", "")
	rec.reset()

	elapsed(3 * time.Hour)
	AlarmCheckDown("vault_sealed", "Vault is sealed", false, "", "")
	if got := len(rec.messages()); got != 0 {
		t.Fatalf("locked alarm repeated inside 24h: %d", got)
	}

	elapsed(72 * time.Hour)
	AlarmCheckDown("vault_sealed", "Vault is sealed", false, "", "")
	if got := len(rec.messages()); got != 1 {
		t.Fatalf("expected one daily reminder after 72h, got %d", got)
	}
}

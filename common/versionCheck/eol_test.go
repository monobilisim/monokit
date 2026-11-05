//go:build linux

package common

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	monocommon "github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/healthdb"
)

// healthdb opens its database once per process, so the tests in this package
// share one, kept out of the user's state dir.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "monokit-versioncheck-test")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_STATE_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// TestCheckLatestVersions_OverlappingRunsAlarmOnce runs checks side by side,
// as happens when cron runs pile up behind a stalled host and resume
// together, and expects the update alarm to go out once.
func TestCheckLatestVersions_OverlappingRunsAlarmOnce(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("as root healthdb uses the system database")
	}
	if err := healthdb.Delete("eol_alarm", "haproxy_eol"); err != nil {
		t.Fatal(err)
	}

	eol := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond) // keep the runs overlapping
		w.Write([]byte(`[{"cycle":"2.6","latest":"2.6.35"}]`))
	}))
	defer eol.Close()

	var alarms atomic.Int32
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alarms.Add(1)
		w.Write([]byte(`{"result":"success"}`))
	}))
	defer webhook.Close()

	oldBase, oldAlarm := eolAPIBase, monocommon.Config.Alarm
	eolAPIBase = eol.URL
	monocommon.Config.Alarm.Enabled = true
	monocommon.Config.Alarm.Webhook_urls = []string{webhook.URL}
	defer func() {
		eolAPIBase = oldBase
		monocommon.Config.Alarm = oldAlarm
	}()

	apps := []AppVersion{{Name: "HAProxy", OldVersion: "2.6.12-1+deb12u3"}}
	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			CheckLatestVersions(apps)
		}()
	}
	wg.Wait()

	if n := alarms.Load(); n != 1 {
		t.Fatalf("sent %d alarms, want 1", n)
	}
}

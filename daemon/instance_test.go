//go:build linux

package daemon

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/shirou/gopsutil/v4/process"
)

// fakeDaemonEnv makes the test binary stand in for a running monokit daemon:
// re-executed with it set, the binary only sleeps, under the argv the test
// gives it.
const fakeDaemonEnv = "MONOKIT_TEST_FAKE_DAEMON"

func TestMain(m *testing.M) {
	if os.Getenv(fakeDaemonEnv) == "1" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// startFakeDaemon starts a process whose command line reads
// "/usr/local/bin/monokit daemon <args>", as cron starts it.
func startFakeDaemon(t *testing.T, args ...string) (*process.Process, <-chan struct{}) {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Args = append([]string{"/usr/local/bin/monokit", "daemon"}, args...)
	cmd.Env = append(os.Environ(), fakeDaemonEnv+"=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-exited
	})

	p, err := process.NewProcess(int32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	return p, exited
}

func TestIsDaemonArgs(t *testing.T) {
	cases := []struct {
		argv []string
		want bool
	}{
		{[]string{"monokit", "daemon"}, true},
		{[]string{"/usr/local/bin/monokit", "daemon", "--once"}, true},
		{[]string{"/usr/local/bin/monokit", "osHealth"}, false},
		{[]string{"/usr/local/bin/monokit", "daemon-test"}, false},
		{[]string{"/usr/local/bin/monokit-old", "daemon"}, false},
		{[]string{"sh", "-c", "monokit daemon --once"}, false},
		{[]string{"monokit"}, false},
	}
	for _, c := range cases {
		if got := isDaemonArgs(c.argv); got != c.want {
			t.Errorf("isDaemonArgs(%q) = %v, want %v", c.argv, got, c.want)
		}
	}
}

func TestClaimDaemonInstance_YieldsToRecentOnceRun(t *testing.T) {
	p, exited := startFakeDaemon(t, "--once")

	if ok, _ := claimDaemonInstance([]*process.Process{p}); ok {
		t.Fatal("expected to yield to a --once run younger than staleOnceRunAge")
	}
	select {
	case <-exited:
		t.Fatal("the running --once run was stopped")
	default:
	}
}

func TestClaimDaemonInstance_YieldsToLongRunningDaemon(t *testing.T) {
	old := staleOnceRunAge
	staleOnceRunAge = -time.Hour // every run counts as stale
	defer func() { staleOnceRunAge = old }()

	p, exited := startFakeDaemon(t)

	if ok, _ := claimDaemonInstance([]*process.Process{p}); ok {
		t.Fatal("expected to yield to a long-running daemon, whatever its age")
	}
	select {
	case <-exited:
		t.Fatal("the long-running daemon was stopped")
	default:
	}
}

func TestClaimDaemonInstance_StopsStaleOnceRun(t *testing.T) {
	old := staleOnceRunAge
	staleOnceRunAge = -time.Hour // every run counts as stale
	defer func() { staleOnceRunAge = old }()

	p, exited := startFakeDaemon(t, "--once")

	ok, hung := claimDaemonInstance([]*process.Process{p})
	if !ok {
		t.Fatal("expected to take over from a stale --once run")
	}
	if len(hung) != 1 || hung[0].pid != p.Pid || hung[0].stopErr != nil {
		t.Fatalf("hung runs = %+v, want pid %d stopped", hung, p.Pid)
	}
	select {
	case <-exited:
	case <-time.After(staleStopTimeout):
		t.Fatal("the stale --once run is still running")
	}
}

func TestAlarmHungRuns_OneAlarmForAllRuns(t *testing.T) {
	var messages []string
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Text string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		messages = append(messages, body.Text)
		w.Write([]byte(`{"result":"success"}`))
	}))
	defer webhook.Close()

	oldAlarm := common.Config.Alarm
	common.Config.Alarm.Enabled = true
	common.Config.Alarm.Webhook_urls = []string{webhook.URL}
	defer func() { common.Config.Alarm = oldAlarm }()

	alarmHungRuns(nil)
	if len(messages) != 0 {
		t.Fatalf("sent %d alarms with no hung runs, want 0", len(messages))
	}

	alarmHungRuns([]hungRun{
		{pid: 101, age: 21 * time.Minute},
		{pid: 102, age: 25 * time.Minute, stopErr: errors.New("operation not permitted")},
	})
	if len(messages) != 1 {
		t.Fatalf("sent %d alarms, want 1", len(messages))
	}
	for _, want := range []string{"pid 101, running for 21 minutes", "pid 102, running for 25 minutes, could not be stopped: operation not permitted"} {
		if !strings.Contains(messages[0], want) {
			t.Errorf("alarm %q does not mention %q", messages[0], want)
		}
	}
}

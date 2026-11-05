package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/process"
)

// staleOnceRunAge is how long a "daemon --once" run may live before the next
// run treats it as hung. A run normally finishes well within a minute. Waiting
// on a hung run forever would stop monitoring on the host; not waiting at all
// lets cron pile up a run every minute while the host is stalled, each one
// sending the same alarms once it resumes. A var so tests can shorten it.
var staleOnceRunAge = 20 * time.Minute

// staleStopTimeout is how long a hung run gets to exit on SIGTERM, which
// cleans up its plugins, before it is killed.
const staleStopTimeout = 10 * time.Second

// isDaemonArgs reports whether argv is a "monokit daemon" invocation, whatever
// path monokit was started from: cron and systemd call it by absolute path.
func isDaemonArgs(argv []string) bool {
	return len(argv) >= 2 && filepath.Base(argv[0]) == "monokit" && argv[1] == "daemon"
}

func isOnceRun(argv []string) bool {
	return slices.Contains(argv, "--once") || slices.Contains(argv, "--once=true")
}

// hungRun is a "daemon --once" run that outlived staleOnceRunAge.
type hungRun struct {
	pid     int32
	age     time.Duration
	stopErr error // why it could not be stopped; this run goes ahead anyway
}

// isOnlyDaemon checks the host's processes for another monokit daemon; see
// claimDaemonInstance.
func isOnlyDaemon() (bool, []hungRun) {
	procs, err := process.Processes()
	if err != nil {
		fmt.Printf("Could not list processes to check for a running daemon: %v\n", err)
	}
	return claimDaemonInstance(procs)
}

// claimDaemonInstance reports whether this process may run as the monokit
// daemon, given the processes running on the host. It yields to a
// long-running daemon and to a --once run younger than staleOnceRunAge. Older
// --once runs are hung: they are stopped, returned for alarmHungRuns, and this
// run takes over.
func claimDaemonInstance(procs []*process.Process) (bool, []hungRun) {
	self := int32(os.Getpid())

	var stale []*process.Process
	var ages []time.Duration
	for _, p := range procs {
		if p.Pid == self {
			continue
		}
		argv, err := p.CmdlineSlice()
		if err != nil || !isDaemonArgs(argv) {
			continue
		}
		if !isOnceRun(argv) {
			fmt.Printf("Monokit daemon is already running (pid %d), exiting...\n", p.Pid)
			return false, nil
		}
		created, err := p.CreateTime()
		age := time.Since(time.UnixMilli(created))
		if err != nil || age < staleOnceRunAge {
			fmt.Printf("Monokit daemon is already running (pid %d), exiting...\n", p.Pid)
			return false, nil
		}
		stale = append(stale, p)
		ages = append(ages, age)
	}

	var hung []hungRun
	for i, p := range stale {
		hung = append(hung, hungRun{pid: p.Pid, age: ages[i], stopErr: stopStaleRun(p)})
	}
	return true, hung
}

// stopStaleRun stops a hung "daemon --once" run: SIGTERM first so it cleans up
// its plugins, then SIGKILL for it and its children if it does not exit.
func stopStaleRun(p *process.Process) error {
	log.Warn().Int32("pid", p.Pid).Dur("max_age", staleOnceRunAge).Msg("Monokit daemon --once run looks hung, stopping it")

	children, _ := p.Children()
	if err := p.Terminate(); err != nil {
		log.Error().Err(err).Int32("pid", p.Pid).Msg("Failed to stop hung monokit daemon --once run")
		return err
	}

	deadline := time.Now().Add(staleStopTimeout)
	for time.Now().Before(deadline) {
		if hasExited(p) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	log.Warn().Int32("pid", p.Pid).Dur("timeout", staleStopTimeout).Msg("Hung monokit daemon --once run ignored SIGTERM, killing it")
	if err := p.Kill(); err != nil {
		log.Error().Err(err).Int32("pid", p.Pid).Msg("Failed to kill hung monokit daemon --once run")
		return err
	}
	for _, c := range children {
		_ = c.Kill()
	}
	return nil
}

// alarmHungRuns sends one alarm for the hung runs claimDaemonInstance replaced.
// It needs the alarm config, so it runs after common.Init.
func alarmHungRuns(hung []hungRun) {
	if len(hung) == 0 {
		return
	}

	runs := make([]string, 0, len(hung))
	for _, r := range hung {
		run := fmt.Sprintf("pid %d, running for %d minutes", r.pid, int(r.age.Minutes()))
		if r.stopErr != nil {
			run += fmt.Sprintf(", could not be stopped: %v", r.stopErr)
		}
		runs = append(runs, run)
	}

	common.Alarm(fmt.Sprintf("[daemon - %s] [:warning:] Hung monokit daemon run replaced (%s); checks on this host did not run meanwhile", common.Config.Identifier, strings.Join(runs, "; ")), "", "", false)
}

func hasExited(p *process.Process) bool {
	if running, err := p.IsRunning(); err != nil || !running {
		return true
	}
	status, err := p.Status()
	return err == nil && slices.Contains(status, process.Zombie)
}

package zimbraHealth

import (
	"reflect"
	"testing"
)

func TestParseZmcontrolStatus_AllRunning(t *testing.T) {
	out := "Host mail.example.com\n" +
		"\tamavis                  Running\n" +
		"\tantispam                Running\n" +
		"\tldap                    Running\n" +
		"\tmailbox                 Running\n" +
		"\tservice webapp          Running\n" +
		"\tzimbra webapp           Running\n" +
		"\tzimbraAdmin webapp      Running\n" +
		"\tzmconfigd               Running\n"

	services, statusMap := parseZmcontrolStatus(out)

	if len(services) != 8 {
		t.Fatalf("expected 8 services, got %d: %+v", len(services), services)
	}
	want := map[string]bool{
		"amavis":             true,
		"antispam":           true,
		"ldap":               true,
		"mailbox":            true,
		"webapp":             true, // "service " prefix stripped
		"zimbra webapp":      true,
		"zimbraAdmin webapp": true,
		"zmconfigd":          true,
	}
	if !reflect.DeepEqual(statusMap, want) {
		t.Errorf("statusMap mismatch:\n got=%+v\nwant=%+v", statusMap, want)
	}
}

// TestParseZmcontrolStatus_StoppedWithMysqlSubLines is the regression test
// for the false-positive incident where the parser invented a phantom
// "mysql.server" service from the indented sub-component line under each
// Stopped entry, producing alarms like "mysql.server is not running".
//
// The sub-component lines must be skipped; only the real top-level
// services (mailbox, webapp, zimbra webapp, etc.) should be reported.
func TestParseZmcontrolStatus_StoppedWithMysqlSubLines(t *testing.T) {
	out := "Host mail.example.com\n" +
		"\tamavis                  Running\n" +
		"\tantispam                Running\n" +
		"\tantivirus               Running\n" +
		"\tcbpolicyd               Running\n" +
		"\tdnscache                Running\n" +
		"\tldap                    Running\n" +
		"\tlogger                  Running\n" +
		"\tmailbox                 Stopped\n" +
		"\t\tmysql.server is not running.\n" +
		"\tmemcached               Running\n" +
		"\tmta                     Running\n" +
		"\topendkim                Running\n" +
		"\tproxy                   Running\n" +
		"\tservice webapp          Stopped\n" +
		"\t\tmysql.server is not running.\n" +
		"\tsnmp                    Running\n" +
		"\tspell                   Running\n" +
		"\tstats                   Running\n" +
		"\tzimbra webapp           Stopped\n" +
		"\t\tmysql.server is not running.\n" +
		"\tzimbraAdmin webapp      Stopped\n" +
		"\t\tmysql.server is not running.\n" +
		"\tzimlet webapp           Stopped\n" +
		"\t\tmysql.server is not running.\n" +
		"\tzmconfigd               Running\n"

	services, statusMap := parseZmcontrolStatus(out)

	if _, ok := statusMap["mysql.server"]; ok {
		t.Fatalf("phantom service 'mysql.server' was parsed; sub-component lines must be skipped. got=%+v", statusMap)
	}

	wantStopped := []string{"mailbox", "webapp", "zimbra webapp", "zimbraAdmin webapp", "zimlet webapp"}
	for _, name := range wantStopped {
		v, ok := statusMap[name]
		if !ok {
			t.Errorf("expected service %q in statusMap, missing. got=%+v", name, statusMap)
			continue
		}
		if v {
			t.Errorf("expected service %q to be Stopped, got Running", name)
		}
	}

	wantRunning := []string{"amavis", "antispam", "antivirus", "cbpolicyd", "dnscache", "ldap", "logger", "memcached", "mta", "opendkim", "proxy", "snmp", "spell", "stats", "zmconfigd"}
	for _, name := range wantRunning {
		v, ok := statusMap[name]
		if !ok {
			t.Errorf("expected service %q in statusMap, missing", name)
			continue
		}
		if !v {
			t.Errorf("expected service %q to be Running, got Stopped", name)
		}
	}

	// 5 stopped + 15 running = 20 services exactly. No more, no less.
	if len(services) != len(wantStopped)+len(wantRunning) {
		t.Errorf("expected %d services, got %d: %+v", len(wantStopped)+len(wantRunning), len(services), services)
	}
}

func TestParseZmcontrolStatus_IgnoresEmptyAndHostHeader(t *testing.T) {
	out := "\n" +
		"Host mail.example.com\n" +
		"\n" +
		"\tldap                    Running\n" +
		"\n"

	services, statusMap := parseZmcontrolStatus(out)
	if len(services) != 1 || !statusMap["ldap"] {
		t.Errorf("expected only 'ldap'=Running, got services=%+v statusMap=%+v", services, statusMap)
	}
}

func TestParseZmcontrolStatus_CarbonioPrefixStripped(t *testing.T) {
	out := "Host mail.example.com\n" +
		"\tcarbonio-mailbox        Running\n" +
		"\tcarbonio-appserver      Stopped\n"

	_, statusMap := parseZmcontrolStatus(out)
	if v, ok := statusMap["mailbox"]; !ok || !v {
		t.Errorf("expected 'mailbox'=Running (carbonio- prefix stripped), got=%+v", statusMap)
	}
	if v, ok := statusMap["appserver"]; !ok || v {
		t.Errorf("expected 'appserver'=Stopped (carbonio- prefix stripped), got=%+v", statusMap)
	}
}

func TestParseZmcontrolStatus_UnknownStatusSkipped(t *testing.T) {
	// "Starting" or any other keyword should be skipped (not Running, not
	// Stopped). The previous implementation's "is not running" branch
	// could mis-classify these; the new last-token check rejects them.
	out := "Host mail.example.com\n" +
		"\tldap                    Running\n" +
		"\tmailbox                 Starting\n"

	services, statusMap := parseZmcontrolStatus(out)
	if _, ok := statusMap["mailbox"]; ok {
		t.Errorf("unknown status keyword 'Starting' must not produce a service entry, got=%+v", statusMap)
	}
	if len(services) != 1 {
		t.Errorf("expected only 1 parsed service, got %d", len(services))
	}
}

func TestParseZmcontrolStatus_EmptyOutput(t *testing.T) {
	services, statusMap := parseZmcontrolStatus("")
	if len(services) != 0 || len(statusMap) != 0 {
		t.Errorf("empty output should produce nothing, got services=%+v statusMap=%+v", services, statusMap)
	}
}

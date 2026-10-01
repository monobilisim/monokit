//go:build linux && plugin

package redisHealth

import "testing"

func TestParseConnectedSlaves(t *testing.T) {
	tests := []struct {
		name  string
		info  string
		count int
		found bool
	}{
		{
			name:  "master with three slaves",
			info:  "# Replication\r\nrole:master\r\nconnected_slaves:3\r\nslave0:ip=10.0.0.2,port=6379,state=online\r\n",
			count: 3,
			found: true,
		},
		{
			name:  "master without slaves",
			info:  "role:master\r\nconnected_slaves:0\r\n",
			count: 0,
			found: true,
		},
		{
			name:  "replica has no connected_slaves field",
			info:  "role:slave\r\nmaster_host:10.0.0.1\r\nmaster_link_status:up\r\n",
			found: false,
		},
		{
			name:  "field present but not a number",
			info:  "connected_slaves:many\r\n",
			found: false,
		},
		{
			name:  "empty info",
			info:  "",
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, found := parseConnectedSlaves(tt.info)
			if found != tt.found {
				t.Fatalf("found = %v, want %v", found, tt.found)
			}
			if found && count != tt.count {
				t.Fatalf("count = %d, want %d", count, tt.count)
			}
		})
	}
}

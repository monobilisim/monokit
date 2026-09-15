package common

import (
	"os"
	"path/filepath"
	"runtime"
)

// KnownPlugins contains the list of all officially supported plugins
var KnownPlugins = []string{
	"k8sHealth", "mysqlHealth", "pgsqlHealth", "redisHealth",
	"zimbraHealth", "traefikHealth", "rmqHealth", "pritunlHealth",
	"wppconnectHealth", "pmgHealth", "esHealth", "postalHealth",
	"mongodbHealth",
}

// DefaultPluginDir is the default directory for plugins. Windows has no
// /var/lib, where a rooted path would land on C:\var\lib, so plugins live
// under ProgramData there.
var DefaultPluginDir = defaultPluginDir()

func defaultPluginDir() string {
	if runtime.GOOS == "windows" {
		if programData := os.Getenv("ProgramData"); programData != "" {
			return filepath.Join(programData, "monokit", "plugins")
		}
		return filepath.Join(os.TempDir(), "monokit", "plugins")
	}
	return "/var/lib/monokit/plugins"
}

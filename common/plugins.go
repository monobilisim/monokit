package common

// KnownPlugins contains the list of all officially supported plugins
var KnownPlugins = []string{
	"k8sHealth", "osHealth", "mysqlHealth", "pgsqlHealth", "redisHealth",
	"zimbraHealth", "traefikHealth", "rmqHealth", "pritunlHealth",
	"wppconnectHealth", "pmgHealth", "esHealth", "postalHealth",
}

// DefaultPluginDir is the default directory for plugins
const DefaultPluginDir = "/var/lib/monokit/plugins"

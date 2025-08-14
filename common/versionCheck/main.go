package common

import (
    "encoding/json"
    "fmt"
    "os"
    "reflect"
    "strings"
    "time"

    "github.com/monobilisim/monokit/common"
    "github.com/monobilisim/monokit/common/health" // For getting plugin providers
    "github.com/monobilisim/monokit/common/healthdb"
    news "github.com/monobilisim/monokit/common/redmine/news"
    "github.com/monobilisim/monokit/common/types" // For RKE2Info struct
    "github.com/rs/zerolog/log"
    "github.com/spf13/cobra"
)

func init() {
	common.RegisterComponent(common.Component{
		Name:       "versionCheck",
		EntryPoint: VersionCheck,
		Platform:   "any", // Assuming it can run anywhere to check external versions
	})
}

func StoreVersion(service string, version string) {
    // Persist version info in SQLite healthdb
    payload := struct {
        Version   string `json:"version"`
        UpdatedAt string `json:"updated_at"`
    }{Version: version, UpdatedAt: time.Now().Format("2006-01-02 15:04:05")}

    b, _ := json.Marshal(payload)
    _ = healthdb.PutJSON("versionCheck", service, string(b), nil, time.Now())

    // Best-effort cleanup of legacy file storage to avoid future confusion
    legacyPath := common.TmpDir + "/" + service + ".version"
    _ = os.Remove(legacyPath)
}

func GatherVersion(service string) string {
    // First, try to read from healthdb
    if jsonStr, _, _, found, err := healthdb.GetJSON("versionCheck", service); err == nil && found && jsonStr != "" {
        // Attempt to parse JSON payload; if parsing fails, assume raw string stored
        var payload struct {
            Version string `json:"version"`
        }
        if json.Unmarshal([]byte(jsonStr), &payload) == nil && payload.Version != "" {
            return payload.Version
        }
        return jsonStr
    }

    // Fallback for legacy storage: read from tmp file once and migrate
    legacyPath := common.TmpDir + "/" + service + ".version"
    if _, err := os.Stat(legacyPath); err == nil {
        if content, err := os.ReadFile(legacyPath); err == nil {
            version := strings.TrimSpace(string(content))
            if version != "" {
                StoreVersion(service, version) // migrate into healthdb (also removes legacy file)
                return version
            }
        }
    }

    return ""
}

func CreateNews(service string, oldVersion string, newVersion string, compactTitle bool) {
	var identifier string = common.Config.Identifier
	if compactTitle {
		parts := strings.Split(identifier, "-")
		if len(parts) > 1 {
			identifier = strings.Join(parts[1:], "-")
		}
	}
	news.Create(identifier+" sunucusunun "+service+" sürümü güncellendi", common.Config.Identifier+" sunucusunda "+service+", "+oldVersion+" sürümünden "+newVersion+" sürümüne yükseltildi.", true)
}

func VersionCheck(cmd *cobra.Command, args []string) {
	version := "0.1.0"
	common.ScriptName = "versionCheck"
	common.TmpDir = "/var/cache/mono/" + common.ScriptName
	common.Init()

	fmt.Println("versionCheck - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05"))

	// Proxmox
	ProxmoxVECheck()
	ProxmoxMGCheck()
	ProxmoxBSCheck()

	// Zimbra
	ZimbraCheck()

	// OPNsense
	OPNsenseCheck()

	// PostgreSQL
	PostgresCheck()

	// MySQL/MariaDB
	MySQLCheck()

	// MongoDB
	MongoDBCheck()

    // Redis
    RedisCheck()

	// RKE2 Kubernetes - Replaced with plugin call
	handleRKE2VersionCheckViaPlugin()

    // Vault
    VaultCheck()

    // FrankenPHP
    FrankenPHPCheck()

    // Caddy
    CaddyCheck()

    // Nginx
    NginxCheck()

    // Docker
    DockerCheck()
}

func handleRKE2VersionCheckViaPlugin() {
	// Attempt RKE2 version check via k8sHealth plugin
	if k8sHealthProvider := health.Get("k8sHealth"); k8sHealthProvider != nil {
		log.Debug().Msg("Attempting RKE2 version check via plugin")

		// Check if the provider supports structured data collection
		if structuredProvider, ok := k8sHealthProvider.(interface {
			CollectStructured(hostname string) (interface{}, error)
		}); ok {
			// Use the new CollectStructured method to get raw JSON data
			jsonData, err := structuredProvider.CollectStructured("") // Hostname might not be relevant for k8sHealth
			if err != nil {
				log.Debug().Err(err).Msg("Error collecting structured data from k8sHealth plugin")
				return
			}

			// Parse the JSON data to extract RKE2Info
			if jsonBytes, ok := jsonData.([]byte); ok {
				var k8sHealthData struct {
					RKE2Info *types.RKE2Info `json:"RKE2Info"`
				}

				if err := json.Unmarshal(jsonBytes, &k8sHealthData); err != nil {
					log.Debug().Err(err).Msg("Error unmarshaling k8sHealth plugin data")
					return
				}

				// Extract and process RKE2 version information
				if k8sHealthData.RKE2Info != nil {
					info := k8sHealthData.RKE2Info

					if info.Error != "" {
						fmt.Printf("RKE2 checker plugin reported an error: %s\n", info.Error)
						return
					}

					if !info.IsRKE2Environment {
						fmt.Println("RKE2 environment not detected by plugin, skipping RKE2 version check.")
						return
					}

					clusterName := info.ClusterName
					if clusterName == "" {
						fmt.Println("Could not determine cluster name for RKE2 version check from plugin.")
						return
					}

					currentVersion := info.CurrentVersion
					if currentVersion == "" {
						fmt.Printf("Could not determine current RKE2 version for cluster '%s' from plugin.\n", clusterName)
						return
					}

					fmt.Printf("RKE2 cluster '%s' version (from plugin): %s\n", clusterName, currentVersion)

					oldVersion := GatherVersion("rke2-" + clusterName)

					if oldVersion != "" && oldVersion == currentVersion {
						fmt.Printf("RKE2 cluster '%s' version is unchanged.\n", clusterName)
						return
					} else if oldVersion != "" && oldVersion != currentVersion {
						fmt.Printf("RKE2 cluster '%s' version has been updated.\n", clusterName)
						fmt.Printf("Old version: %s\n", oldVersion)
						fmt.Printf("New version: %s\n", currentVersion)

						if info.IsMasterNode {
							CreateNews("RKE2", oldVersion, currentVersion, false)
						} else {
							log.Debug().Msg("Skipping news creation (not a master node)")
						}
					} else { // oldVersion == ""
						fmt.Printf("First time detecting RKE2 cluster '%s' version via plugin.\n", clusterName)
					}

					StoreVersion("rke2-"+clusterName, currentVersion)
				} else {
					log.Debug().Msg("No RKE2Info found in plugin data")
				}
			} else {
				log.Debug().Interface("data", jsonData).Msg("Unexpected structured data type from k8sHealth plugin")
			}
		} else {
			// Fallback to old behavior for plugins that don't support structured data
			log.Debug().Msg("Plugin doesn't support structured data collection, falling back to rendered output")
			pluginData, err := k8sHealthProvider.Collect("")
			if err != nil {
				log.Debug().Err(err).Msg("Error collecting data from k8sHealth plugin")
				return
			}

			// The k8sHealth plugin now returns a pre-rendered string, not a struct
			// Skip RKE2 version checking via plugin since it returns rendered output
			if _, ok := pluginData.(string); ok {
				log.Debug().Msg("k8sHealth plugin returned string output, skipping RKE2 version extraction")
				fmt.Println("RKE2 version check via plugin skipped (plugin returns rendered output)")
				return
			}

			// Keep the existing struct-based logic as fallback for backward compatibility
			// Use reflection to access the RKE2Info field since we can't import k8sHealth due to build constraints
			v := reflect.ValueOf(pluginData)
			if v.Kind() == reflect.Ptr {
				v = v.Elem()
			}

			if v.Kind() != reflect.Struct {
				fmt.Printf("Unexpected data type from k8sHealth plugin: expected struct, got %T\n", pluginData)
				return
			}

			rke2InfoField := v.FieldByName("RKE2Info")
			if !rke2InfoField.IsValid() {
				log.Debug().Msg("No RKE2Info field found in k8sHealth plugin response")
				return
			}

			if rke2InfoField.IsNil() {
				log.Debug().Msg("No RKE2 information available from k8sHealth plugin")
				return
			}

			// Extract the RKE2Info using reflection
			rke2InfoValue := rke2InfoField.Elem()
			if rke2InfoValue.Kind() != reflect.Struct {
				fmt.Printf("RKE2Info field has unexpected type: %v\n", rke2InfoValue.Kind())
				return
			}

			// Extract fields from the RKE2Info struct using reflection
			info := types.RKE2Info{
				IsRKE2Environment: rke2InfoValue.FieldByName("IsRKE2Environment").Bool(),
				ClusterName:       rke2InfoValue.FieldByName("ClusterName").String(),
				CurrentVersion:    rke2InfoValue.FieldByName("CurrentVersion").String(),
				IsMasterNode:      rke2InfoValue.FieldByName("IsMasterNode").Bool(),
				Error:             rke2InfoValue.FieldByName("Error").String(),
			}

			if info.Error != "" {
				fmt.Printf("RKE2 checker plugin reported an error: %s\n", info.Error)
				// Decide if we should still proceed if only partial data is available
			}

			if !info.IsRKE2Environment {
				fmt.Println("RKE2 environment not detected by plugin, skipping RKE2 version check.")
				return
			}

			clusterName := info.ClusterName
			if clusterName == "" {
				fmt.Println("Could not determine cluster name for RKE2 version check from plugin.")
				return
			}

			currentVersion := info.CurrentVersion
			if currentVersion == "" {
				fmt.Printf("Could not determine current RKE2 version for cluster '%s' from plugin.\n", clusterName)
				return
			}

			fmt.Printf("RKE2 cluster '%s' version (from plugin): %s\n", clusterName, currentVersion)

			oldVersion := GatherVersion("rke2-" + clusterName)

			if oldVersion != "" && oldVersion == currentVersion {
				fmt.Printf("RKE2 cluster '%s' version is unchanged.\n", clusterName)
				return
			} else if oldVersion != "" && oldVersion != currentVersion {
				fmt.Printf("RKE2 cluster '%s' version has been updated.\n", clusterName)
				fmt.Printf("Old version: %s\n", oldVersion)
				fmt.Printf("New version: %s\n", currentVersion)

				if info.IsMasterNode {
					// Using existing CreateNews function from this file.
					// It uses common.Config.Identifier for node identification in news.
					CreateNews("RKE2", oldVersion, currentVersion, false)
				} else {
					log.Debug().Msg("Skipping news creation (not a master node, or plugin indicated so)")
				}
			} else { // oldVersion == ""
				fmt.Printf("First time detecting RKE2 cluster '%s' version via plugin.\n", clusterName)
			}

			StoreVersion("rke2-"+clusterName, currentVersion)
		}
	} else {
		log.Debug().Msg("k8sHealth plugin not found or not loaded")
		return
	}
}

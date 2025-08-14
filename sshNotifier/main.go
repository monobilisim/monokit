package sshNotifier

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/healthdb"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

// TODO(migration): Deprecated fields will be removed after all configs migrate
// from file-based monitoring to DB-based monitoring.
var SSHNotifierConfig struct {
	Exclude struct {
		Domains []string
		IPs     []string
		Users   []string
	}

	Server struct {
		Os_Type string
		Address string
	}

	Ssh_Post_Url        string
	Ssh_Post_Url_Backup string

	Webhook struct {
		Stream string
		Topic  string
	}

	// New canonical fields
	Disable_Db_Monitoring bool
	SkippedComponents     []string

	// Deprecated (backwards compatibility):
	// TODO(migration): Remove these after safe rollout
	Disable_File_Monitoring bool
	SkippedDirectories      []string
}

type LoginInfoOutput struct {
	Username    string `json:"username"`
	Fingerprint string `json:"fingerprint"`
	Server      string `json:"server"`
	RemoteIp    string `json:"remote_ip"`
	Date        string `json:"date"`
	EventType   string `json:"event_type"`
	LoginMethod string `json:"login_method"`
	Ppid        string `json:"ppid"`
	PamUser     string `json:"pam_user"`
}

type DatabaseRequest struct {
	Ppid          string `json:"PPID"`
	LinuxUser     string `json:"linux_user"`
	Type          string `json:"type"`
	KeyComment    string `json:"key_comment"`
	Host          string `json:"host"`
	ConnectedFrom string `json:"connected_from"`
	LoginType     string `json:"login_type"`
}

func Grep(pattern string, contents string) string {
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "grep").
		Str("pattern", pattern).
		Int("content_length", len(contents)).
		Msg("Searching for pattern in content")

	scanner := bufio.NewScanner(strings.NewReader(contents))
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), pattern) {
			foundLine := scanner.Text()
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "grep").
				Str("pattern", pattern).
				Str("found_line", foundLine).
				Msg("Pattern found in content")
			return foundLine
		}
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "grep").
		Str("pattern", pattern).
		Msg("Pattern not found in content")
	return ""
}

func getSHA256Fingerprint(rawKey string) string {
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "fingerprint_generation").
		Int("raw_key_length", len(rawKey)).
		Msg("Generating SHA256 fingerprint from raw key")

	// Parse the public key
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(rawKey))
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "sshNotifier").
			Str("operation", "fingerprint_generation").
			Str("action", "parse_key").
			Int("raw_key_length", len(rawKey)).
			Msg("Failed to parse SSH public key")
		return ""
	}

	// Get the raw key bytes
	keyBytes := pubKey.Marshal()

	// Calculate SHA256 hash
	hash := sha256.Sum256(keyBytes)

	// Convert to base64
	b64Hash := base64.StdEncoding.EncodeToString(hash[:])

	// Remove padding characters
	b64Hash = strings.TrimRight(b64Hash, "=")

	// Format with SHA256: prefix
	fingerprint := "SHA256:" + b64Hash

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "fingerprint_generation").
		Str("action", "generated").
		Str("fingerprint", fingerprint).
		Int("key_bytes_length", len(keyBytes)).
		Msg("Successfully generated SHA256 fingerprint")

	return fingerprint
}

func getFingerprintFromAuthLog(ppid string) string {
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "fingerprint_lookup").
		Str("action", "search_auth_log").
		Str("ppid", ppid).
		Msg("Searching for fingerprint in authentication logs")

	// Check if /var/log/secure exists
	var logFile string
	if _, err := os.Stat("/var/log/secure"); os.IsNotExist(err) {
		logFile = "/var/log/auth.log"
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "fingerprint_lookup").
			Str("action", "log_file_selection").
			Str("log_file", logFile).
			Str("reason", "secure_not_found").
			Msg("Using auth.log as log file")
	} else {
		logFile = "/var/log/secure"
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "fingerprint_lookup").
			Str("action", "log_file_selection").
			Str("log_file", logFile).
			Str("reason", "secure_found").
			Msg("Using secure as log file")
	}

	keyword := "Accepted publickey"
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "fingerprint_lookup").
		Str("action", "search_setup").
		Str("keyword", keyword).
		Str("ppid", ppid).
		Msg("Setting up authentication log search")

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		log.Error().
			Err(err).
			Str("component", "sshNotifier").
			Str("operation", "fingerprint_lookup").
			Str("action", "file_access").
			Str("log_file", logFile).
			Msg("Authentication log file does not exist")
		return ""
	}

	// Read the log file
	file, err := os.ReadFile(logFile)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "sshNotifier").
			Str("operation", "fingerprint_lookup").
			Str("action", "file_read").
			Str("log_file", logFile).
			Msg("Failed to read authentication log file")
		return ""
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "fingerprint_lookup").
		Str("action", "file_read").
		Str("log_file", logFile).
		Int("file_size", len(file)).
		Msg("Successfully read authentication log file")

	// Extract fingerprint from log file similar to: grep "$keyword" "$logfile" | grep $PPID | tail -n 1 | awk '{print $NF}'
	lines := strings.Split(string(file), "\n")
	var matchingLines []string

	// Find lines containing both keyword and PPID
	for _, line := range lines {
		if strings.Contains(line, keyword) && strings.Contains(line, ppid) {
			matchingLines = append(matchingLines, line)
		}
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "fingerprint_lookup").
		Str("action", "search_results").
		Str("keyword", keyword).
		Str("ppid", ppid).
		Int("matching_lines", len(matchingLines)).
		Msg("Found matching lines in authentication log")

	// Get the last matching line if any found
	if len(matchingLines) > 0 {
		lastLine := matchingLines[len(matchingLines)-1]
		// Extract the last field (equivalent to awk '{print $NF}')
		fields := strings.Fields(lastLine)
		if len(fields) > 0 {
			fingerprint := fields[len(fields)-1]
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "fingerprint_lookup").
				Str("action", "extraction_success").
				Str("fingerprint", fingerprint).
				Str("ppid", ppid).
				Int("fields_count", len(fields)).
				Msg("Successfully extracted fingerprint from authentication log")
			return fingerprint
		}
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "fingerprint_lookup").
		Str("action", "extraction_failed").
		Str("keyword", keyword).
		Str("ppid", ppid).
		Msg("No fingerprint found in authentication log")
	return ""
}

func GetLoginInfo(customType string) LoginInfoOutput {
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "login_info_collection").
		Str("action", "start").
		Str("custom_type", customType).
		Msg("Starting login information collection")

	var eventType string

	// If no eventType provided, determine it from PAM_TYPE environment variable
	if customType == "" {
		eventType = os.Getenv("PAM_TYPE")
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "login_info_collection").
			Str("action", "event_type_detection").
			Str("pam_type", eventType).
			Str("source", "environment").
			Msg("Event type determined from PAM_TYPE environment variable")
	} else {
		eventType = customType
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "login_info_collection").
			Str("action", "event_type_detection").
			Str("event_type", eventType).
			Str("source", "parameter").
			Msg("Event type provided as parameter")
	}

	// Ensure eventType is set to a default if it's still empty
	if eventType == "" {
		eventType = "unknown"
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "login_info_collection").
			Str("action", "event_type_fallback").
			Str("event_type", eventType).
			Msg("Event type was empty, using default")
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "login_info_collection").
		Str("action", "event_type_finalized").
		Str("event_type", eventType).
		Msg("Final event type determined")

	var loginMethod string
	var fingerprint string
	var ppid string
	var authorizedKeys string
	var username string

	ppid = strconv.Itoa(os.Getppid())
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "login_info_collection").
		Str("action", "ppid_detection").
		Str("ppid", ppid).
		Msg("Parent process ID detected")

	// First try to get fingerprint from SSH_AUTH_INFO_0 environment variable
	sshAuthInfo := os.Getenv("SSH_AUTH_INFO_0")
	if sshAuthInfo != "" {
		// Trim any whitespace and newlines
		sshAuthInfo = strings.TrimSpace(sshAuthInfo)
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "login_info_collection").
			Str("action", "ssh_auth_info_found").
			Int("ssh_auth_info_length", len(sshAuthInfo)).
			Msg("Found SSH_AUTH_INFO_0 environment variable")

		// SSH_AUTH_INFO_0 format: "publickey ssh-ed25519 key"
		fields := strings.Fields(sshAuthInfo)
		if len(fields) >= 3 {
			fingerprint = getSHA256Fingerprint(sshAuthInfo) // Pass the full key string
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "login_info_collection").
				Str("action", "fingerprint_conversion").
				Str("fingerprint", fingerprint).
				Int("fields_count", len(fields)).
				Msg("Converted raw key to SHA256 fingerprint from SSH_AUTH_INFO_0")
		} else {
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "login_info_collection").
				Str("action", "ssh_auth_info_invalid").
				Int("fields_count", len(fields)).
				Msg("SSH_AUTH_INFO_0 has insufficient fields")
		}
	} else {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "login_info_collection").
			Str("action", "ssh_auth_info_missing").
			Msg("SSH_AUTH_INFO_0 environment variable not found")
	}

	// If no fingerprint from SSH_AUTH_INFO_0, try auth.log
	if fingerprint == "" {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "login_info_collection").
			Str("action", "fallback_fingerprint_lookup").
			Str("ppid", ppid).
			Msg("No fingerprint from SSH_AUTH_INFO_0, searching authentication logs")
		fingerprint = getFingerprintFromAuthLog(ppid)
	}

	if fingerprint == "" {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "login_info_collection").
			Str("action", "fingerprint_not_found").
			Str("ppid", ppid).
			Msg("No fingerprint found from any source, will try PAM_USER matching")
	}

	pamUser := os.Getenv("PAM_USER")
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "login_info_collection").
		Str("action", "pam_user_detection").
		Str("pam_user", pamUser).
		Msg("PAM_USER environment variable detected")

	if pamUser == "root" {
		authorizedKeys = "/root/.ssh/authorized_keys"
	} else {
		authorizedKeys = "/home/" + pamUser + "/.ssh/authorized_keys"
	}
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "login_info_collection").
		Str("action", "authorized_keys_path").
		Str("authorized_keys", authorizedKeys).
		Str("pam_user", pamUser).
		Msg("Determined authorized_keys file path")

	// Try first to determine if this is an SSH key login by checking logs for fingerprint
	var sshKeyLogin bool = false

	if _, err := os.Stat(authorizedKeys); err == nil {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "login_info_collection").
			Str("action", "authorized_keys_check").
			Str("authorized_keys", authorizedKeys).
			Bool("exists", true).
			Msg("Authorized keys file exists, proceeding with key analysis")

		if SSHNotifierConfig.Server.Os_Type == "GENERIC" {
			keysOut, err := exec.Command("/usr/bin/ssh-keygen", "-lf", authorizedKeys).Output()
			if err != nil {
				log.Error().
					Err(err).
					Str("component", "sshNotifier").
					Str("operation", "login_info_collection").
					Str("action", "ssh_keygen_execution").
					Str("authorized_keys", authorizedKeys).
					Msg("Failed to execute ssh-keygen command")
				return LoginInfoOutput{}
			}

			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "login_info_collection").
				Str("action", "ssh_keygen_output").
				Int("output_length", len(keysOut)).
				Msg("ssh-keygen command executed successfully")

			// Split output into lines
			keysOutSplit := strings.Split(string(keysOut), "\n")

			// Check each line for fingerprint match
			if fingerprint != "" {
				log.Debug().
					Str("component", "sshNotifier").
					Str("operation", "login_info_collection").
					Str("action", "fingerprint_matching").
					Str("fingerprint", fingerprint).
					Int("key_lines", len(keysOutSplit)).
					Msg("Searching for matching fingerprint in authorized keys")

				for _, key := range keysOutSplit {
					if len(key) == 0 {
						continue
					}

					if strings.Contains(key, fingerprint) {
						log.Debug().
							Str("component", "sshNotifier").
							Str("operation", "login_info_collection").
							Str("action", "fingerprint_match_found").
							Str("fingerprint", fingerprint).
							Str("key_line", key).
							Msg("Found matching fingerprint in authorized keys")

						// Extract the third field (username)
						fields := strings.Fields(key)
						log.Debug().
							Str("component", "sshNotifier").
							Str("operation", "login_info_collection").
							Str("action", "username_extraction").
							Int("fields_count", len(fields)).
							Msg("Extracting username from key line")

						if len(fields) >= 3 {
							username = fields[2]
							loginMethod = "ssh-key"
							sshKeyLogin = true
							log.Debug().
								Str("component", "sshNotifier").
								Str("operation", "login_info_collection").
								Str("action", "ssh_key_login_confirmed").
								Str("username", username).
								Str("login_method", loginMethod).
								Msg("SSH key login confirmed with username")
							break
						}
					}
				}
			} else {
				log.Debug().
					Str("component", "sshNotifier").
					Str("operation", "login_info_collection").
					Str("action", "no_fingerprint_for_matching").
					Msg("No fingerprint available for matching against authorized keys")
			}
		} else {
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "login_info_collection").
				Str("action", "os_type_skip").
				Str("os_type", SSHNotifierConfig.Server.Os_Type).
				Msg("Skipping ssh-keygen analysis due to OS type")
		}
	} else {
		username = pamUser
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "login_info_collection").
			Str("action", "authorized_keys_not_found").
			Str("authorized_keys", authorizedKeys).
			Str("fallback_username", username).
			Msg("Authorized keys file not found, using PAM_USER as username")
	}

	var userTmp string

	// Check exclusions
	for _, excludeUser := range SSHNotifierConfig.Exclude.Users {
		userTmp = username
		if userTmp == "" {
			userTmp = pamUser
		}

		if strings.Contains(userTmp, "@") {
			userTmp = strings.Split(userTmp, "@")[0]
		}

		if userTmp == excludeUser {
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "login_info_collection").
				Str("action", "user_excluded").
				Str("excluded_user", excludeUser).
				Str("detected_user", userTmp).
				Msg("User is in exclusion list, returning empty result")
			return LoginInfoOutput{}
		}
	}

	for _, excludeIp := range SSHNotifierConfig.Exclude.IPs {
		remoteHost := os.Getenv("PAM_RHOST")
		if remoteHost == excludeIp && remoteHost != "" {
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "login_info_collection").
				Str("action", "ip_excluded").
				Str("excluded_ip", excludeIp).
				Str("remote_ip", remoteHost).
				Msg("Remote IP is in exclusion list, returning empty result")
			return LoginInfoOutput{}
		}
	}

	for _, excludeDomain := range SSHNotifierConfig.Exclude.Domains {
		if strings.Contains(userTmp, excludeDomain) && userTmp != "" {
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "login_info_collection").
				Str("action", "domain_excluded").
				Str("excluded_domain", excludeDomain).
				Str("detected_user", userTmp).
				Msg("User domain is in exclusion list, returning empty result")
			return LoginInfoOutput{}
		}
	}

	// Set login method based on our determination
	if sshKeyLogin {
		loginMethod = "ssh-key"
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "login_info_collection").
			Str("action", "login_method_determined").
			Str("login_method", loginMethod).
			Str("reason", "key_authentication").
			Msg("Login method set to SSH key authentication")
	} else {
		loginMethod = "password"
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "login_info_collection").
			Str("action", "login_method_determined").
			Str("login_method", loginMethod).
			Str("reason", "fallback").
			Msg("Login method set to password authentication")
	}

	result := LoginInfoOutput{
		Username:    username,
		Fingerprint: fingerprint,
		Server:      pamUser + "@" + common.Config.Identifier,
		RemoteIp:    os.Getenv("PAM_RHOST"),
		Date:        time.Now().Format("02.01.2006 15:04:05"),
		EventType:   eventType,
		LoginMethod: loginMethod,
		Ppid:        ppid,
		PamUser:     pamUser,
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "login_info_collection").
		Str("action", "completed").
		Str("username", result.Username).
		Str("fingerprint", result.Fingerprint).
		Str("server", result.Server).
		Str("remote_ip", result.RemoteIp).
		Str("event_type", result.EventType).
		Str("login_method", result.LoginMethod).
		Str("ppid", result.Ppid).
		Str("pam_user", result.PamUser).
		Msg("Login information collection completed successfully")

	return result
}

// TODO(migration): This file-based logic is deprecated; keep until all nodes use DB-based detection.
func listFiles(dir string, ignoredDirectories []string) []string {
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "file_monitoring").
		Str("action", "list_files").
		Str("directory", dir).
		// TODO(migration): drop deprecated field after migration
		Bool("monitoring_disabled", SSHNotifierConfig.Disable_File_Monitoring || SSHNotifierConfig.Disable_Db_Monitoring).
		Int("ignored_directories", len(ignoredDirectories)).
		Msg("Starting file listing operation")

	// Backward-compat: consider Disable_File_Monitoring OR Disable_Db_Monitoring
	if SSHNotifierConfig.Disable_File_Monitoring || SSHNotifierConfig.Disable_Db_Monitoring {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "file_monitoring").
			Str("action", "disabled").
			Msg("File monitoring is disabled in configuration")
		return []string{}
	}

	// Check if the directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "file_monitoring").
			Str("action", "directory_not_found").
			Str("directory", dir).
			Msg("Directory does not exist, returning empty list")
		return []string{}
	}

	var files []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Error().
				Err(err).
				Str("component", "sshNotifier").
				Str("operation", "file_monitoring").
				Str("action", "walk_error").
				Str("path", path).
				Msg("Error encountered while walking directory")
			return err
		}

		// Skip ignored directories
		if d.IsDir() {
			for _, ignoredDir := range ignoredDirectories {
				if strings.Contains(path, ignoredDir) {
					log.Debug().
						Str("component", "sshNotifier").
						Str("operation", "file_monitoring").
						Str("action", "directory_skipped").
						Str("path", path).
						Str("ignored_directory", ignoredDir).
						Msg("Skipping ignored directory")
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Only include files with .log extension
		if filepath.Ext(path) == ".log" {
			files = append(files, path)
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "file_monitoring").
				Str("action", "file_added").
				Str("file_path", path).
				Msg("Added log file to monitoring list")
		}
		return nil
	})

	if err != nil {
		log.Error().
			Err(err).
			Str("component", "sshNotifier").
			Str("operation", "file_monitoring").
			Str("action", "walk_failed").
			Str("directory", dir).
			Msg("Failed to walk directory tree")
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "file_monitoring").
		Str("action", "completed").
		Str("directory", dir).
		Int("files_found", len(files)).
		Msg("File listing operation completed")

	return files
}

// TODO(migration): Deprecated file discovery; retained only during migration.
func getRelevantLogs(skippedDirectories []string) []string {
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "get_relevant_logs").
		Str("action", "start").
		Msg("Starting to get relevant logs")

	// Backward-compat: consider Disable_File_Monitoring OR Disable_Db_Monitoring
	if SSHNotifierConfig.Disable_File_Monitoring || SSHNotifierConfig.Disable_Db_Monitoring {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "get_relevant_logs").
			Str("action", "disabled").
			Msg("File monitoring is disabled in configuration")
		return []string{}
	}

	componentsStr := common.GetInstalledComponents()
	if componentsStr == "" {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "get_relevant_logs").
			Str("action", "no_components").
			Msg("No installed components found")
		return []string{}
	}

	components := strings.Split(componentsStr, "::")
	var allLogFiles []string

	for _, component := range components {
		// Use basename-like behavior to ensure component names without path segments
		// TODO(migration): remove basename normalization once we're confident only names are present
		baseComp := filepath.Base(component)
		componentDir := filepath.Join("/tmp/mono", baseComp)
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "get_relevant_logs").
			Str("action", "checking_component").
			Str("component_name", baseComp).
			Str("directory", componentDir).
			Msg("Checking component directory for log files")

		if _, err := os.Stat(componentDir); os.IsNotExist(err) {
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "get_relevant_logs").
				Str("action", "directory_not_found").
				Str("directory", componentDir).
				Msg("Component directory does not exist, skipping")
			continue
		}

		files := listFiles(componentDir, skippedDirectories)
		allLogFiles = append(allLogFiles, files...)
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "get_relevant_logs").
		Str("action", "completed").
		Int("total_logs_found", len(allLogFiles)).
		Msg("Finished getting relevant logs")

	return allLogFiles
}
func PostToDb(postUrl string, dbReq DatabaseRequest) error {
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "database_post").
		Str("action", "start").
		Str("post_url", postUrl).
		Str("ppid", dbReq.Ppid).
		Str("linux_user", dbReq.LinuxUser).
		Str("type", dbReq.Type).
		Msg("Starting database post operation")

	// Marshal the struct to JSON
	jsonReq, err := json.Marshal(dbReq)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "sshNotifier").
			Str("operation", "database_post").
			Str("action", "json_marshal").
			Str("post_url", postUrl).
			Msg("Failed to marshal database request to JSON")
		return err
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "database_post").
		Str("action", "json_marshaled").
		Int("json_size", len(jsonReq)).
		Msg("Successfully marshaled database request to JSON")

	req, err := http.NewRequest("POST", postUrl, bytes.NewBuffer(jsonReq))
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "sshNotifier").
			Str("operation", "database_post").
			Str("action", "request_creation").
			Str("post_url", postUrl).
			Msg("Failed to create HTTP request")
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: time.Second,
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "database_post").
		Str("action", "sending_request").
		Str("post_url", postUrl).
		Str("method", "POST").
		Str("content_type", "application/json").
		Msg("Sending HTTP request to database")

	resp, err := client.Do(req)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "sshNotifier").
			Str("operation", "database_post").
			Str("action", "request_failed").
			Str("post_url", postUrl).
			Msg("HTTP request to database failed")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Error().
			Str("component", "sshNotifier").
			Str("operation", "database_post").
			Str("action", "non_200_response").
			Str("post_url", postUrl).
			Int("status_code", resp.StatusCode).
			Str("status", resp.Status).
			Msg("Database server returned non-200 status code")
		return fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "database_post").
		Str("action", "success").
		Str("post_url", postUrl).
		Int("status_code", resp.StatusCode).
		Msg("Successfully posted data to database")
	return nil
}

func NotifyAndSave(loginInfo LoginInfoOutput) {
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "notification_and_save").
		Str("action", "start").
		Str("event_type", loginInfo.EventType).
		Str("username", loginInfo.Username).
		Str("remote_ip", loginInfo.RemoteIp).
		Str("ppid", loginInfo.Ppid).
		Msg("Starting notification and save process")

	if loginInfo.EventType == "" {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "notification_and_save").
			Str("action", "empty_event_type").
			Msg("Event type is empty, skipping notification and save process")
		return
	}

	var message string
	var isLoginEvent bool

	if loginInfo.EventType == "open_session" {
		isLoginEvent = true
		message = "[ " + common.Config.Identifier + " ] " + "[ :green_circle: Login ] { " + loginInfo.Username + "@" + loginInfo.RemoteIp + " } >> { " + SSHNotifierConfig.Server.Address + " - " + loginInfo.Ppid + " }"
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "notification_and_save").
			Str("action", "login_event").
			Str("username", loginInfo.Username).
			Str("remote_ip", loginInfo.RemoteIp).
			Str("server_address", SSHNotifierConfig.Server.Address).
			Str("ppid", loginInfo.Ppid).
			Msg("Processing login event")
	} else {
		isLoginEvent = false
		message = "[ " + common.Config.Identifier + " ] " + "[ :red_circle: Logout ] { " + loginInfo.Username + "@" + loginInfo.RemoteIp + " } << { " + SSHNotifierConfig.Server.Address + " - " + loginInfo.Ppid + " }"
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "notification_and_save").
			Str("action", "logout_event").
			Str("username", loginInfo.Username).
			Str("remote_ip", loginInfo.RemoteIp).
			Str("server_address", SSHNotifierConfig.Server.Address).
			Str("ppid", loginInfo.Ppid).
			Msg("Processing logout event")
	}

	// Preserve original username for KeyComment field
	originalUsername := loginInfo.Username

	if strings.Contains(loginInfo.Username, "@") {
		cleanedUsername := strings.Split(loginInfo.Username, "@")[0]
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "notification_and_save").
			Str("action", "username_cleanup").
			Str("original_username", loginInfo.Username).
			Str("cleaned_username", cleanedUsername).
			Msg("Cleaned username by removing domain part")
		loginInfo.Username = cleanedUsername
	}

	// Determine whether there is active monitoring by checking SQLite alarm entries
	if SSHNotifierConfig.Disable_Db_Monitoring {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "notification_and_save").
			Str("action", "db_monitoring_disabled").
			Msg("DB monitoring disabled by configuration - skipping DB lookup and using webhook")
		if SSHNotifierConfig.Webhook.Stream == "" {
			var topic string
			if SSHNotifierConfig.Webhook.Topic != "" {
				topic = SSHNotifierConfig.Webhook.Topic
			} else {
				topic = ""
			}

			common.Alarm(message, "", topic, false)
		} else {
			var usernameOnStream string
			if strings.Contains(loginInfo.Username, "@") {
				usernameOnStream = strings.Split(loginInfo.Username, "@")[0]
			} else {
				usernameOnStream = loginInfo.Username
			}

			if SSHNotifierConfig.Webhook.Topic != "" {
				usernameOnStream = SSHNotifierConfig.Webhook.Topic
			}

			common.Alarm(message, SSHNotifierConfig.Webhook.Stream, usernameOnStream, true)
		}
		// continue to DB post below
	} else {
		alarmCount := countRecentAlarmsInDB(24 * time.Hour)

		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "notification_and_save").
			Str("action", "db_alarm_check").
			Int("recent_alarms", alarmCount).
			Msg("Checked recent alarms in SQLite database")

		if alarmCount == 0 {
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "notification_and_save").
				Str("action", "webhook_notification_no_recent_alarms").
				Str("webhook_stream", SSHNotifierConfig.Webhook.Stream).
				Bool("use_stream", SSHNotifierConfig.Webhook.Stream != "").
				Msg("No recent alarms found in DB, using webhook configuration")

			if SSHNotifierConfig.Webhook.Stream == "" {
				common.Alarm(message, "", "", false)
			} else {
				var usernameOnStream string
				if strings.Contains(loginInfo.Username, "@") {
					usernameOnStream = strings.Split(loginInfo.Username, "@")[0]
				} else {
					usernameOnStream = loginInfo.Username
				}

				log.Debug().
					Str("component", "sshNotifier").
					Str("operation", "notification_and_save").
					Str("action", "stream_notification").
					Str("stream", SSHNotifierConfig.Webhook.Stream).
					Str("username_on_stream", usernameOnStream).
					Msg("Sending notification with stream configuration")

				common.Alarm(message, SSHNotifierConfig.Webhook.Stream, usernameOnStream, true)
			}
		} else {
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "notification_and_save").
				Str("action", "standard_alarm_recent_alarms_present").
				Int("recent_alarms", alarmCount).
				Msg("Recent alarms exist in DB, sending standard alarm")
			common.Alarm(message, "", "", false)
		}
	}

	var dbReq DatabaseRequest
	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "notification_and_save").
		Str("action", "database_request_preparation").
		Msg("Preparing database request")

	dbReq.Ppid = "'" + loginInfo.Ppid + "'"
	dbReq.LinuxUser = "'" + loginInfo.PamUser + "'"
	dbReq.Type = "'" + loginInfo.EventType + "'"
	dbReq.KeyComment = "'" + originalUsername + "'"
	dbReq.Host = "'" + loginInfo.Server + "'"
	dbReq.ConnectedFrom = "'" + loginInfo.RemoteIp + "'"
	dbReq.LoginType = "'" + loginInfo.LoginMethod + "'"

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "notification_and_save").
		Str("action", "database_request_prepared").
		Str("ppid", dbReq.Ppid).
		Str("linux_user", dbReq.LinuxUser).
		Str("type", dbReq.Type).
		Str("key_comment", dbReq.KeyComment).
		Str("host", dbReq.Host).
		Str("connected_from", dbReq.ConnectedFrom).
		Str("login_type", dbReq.LoginType).
		Msg("Database request prepared with all fields")

	// Security logging for authentication events
	log.Info().
		Str("component", "sshNotifier").
		Str("operation", "security_event").
		Str("action", "authentication_attempt").
		Str("event_type", loginInfo.EventType).
		Str("username", loginInfo.PamUser).
		Str("remote_ip", loginInfo.RemoteIp).
		Str("login_method", loginInfo.LoginMethod).
		Str("server_identifier", common.Config.Identifier).
		Str("ppid", loginInfo.Ppid).
		Bool("is_login", isLoginEvent).
		Bool("has_fingerprint", loginInfo.Fingerprint != "").
		Msg("Security event logged for authentication attempt")

	err := PostToDb(SSHNotifierConfig.Ssh_Post_Url, dbReq)
	if err != nil {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "notification_and_save").
			Str("action", "primary_db_failed").
			Str("primary_url", SSHNotifierConfig.Ssh_Post_Url).
			Str("backup_url", SSHNotifierConfig.Ssh_Post_Url_Backup).
			Msg("Primary database post failed, attempting backup URL")

		err = PostToDb(SSHNotifierConfig.Ssh_Post_Url_Backup, dbReq)
		if err != nil {
			log.Error().
				Err(err).
				Str("component", "sshNotifier").
				Str("operation", "notification_and_save").
				Str("action", "all_db_failed").
				Str("primary_url", SSHNotifierConfig.Ssh_Post_Url).
				Str("backup_url", SSHNotifierConfig.Ssh_Post_Url_Backup).
				Msg("Both primary and backup database posts failed")
		} else {
			log.Debug().
				Str("component", "sshNotifier").
				Str("operation", "notification_and_save").
				Str("action", "backup_db_success").
				Str("backup_url", SSHNotifierConfig.Ssh_Post_Url_Backup).
				Msg("Successfully posted to backup database")
		}
	} else {
		log.Debug().
			Str("component", "sshNotifier").
			Str("operation", "notification_and_save").
			Str("action", "primary_db_success").
			Str("primary_url", SSHNotifierConfig.Ssh_Post_Url).
			Msg("Successfully posted to primary database")
	}

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "notification_and_save").
		Str("action", "completed").
		Str("event_type", loginInfo.EventType).
		Str("username", loginInfo.Username).
		Str("remote_ip", loginInfo.RemoteIp).
		Bool("notification_sent", true).
		Bool("database_attempted", true).
		Msg("Notification and save process completed")
}

// countRecentAlarmsInDB returns the number of alarm records in SQLite within the given duration.
// If duration is zero or negative, counts all alarm records.
func countRecentAlarmsInDB(since time.Duration) int {
	db := healthdb.Get()
	var cnt int64
	if since > 0 {
		sinceTime := time.Now().Add(-since)
		_ = db.Model(&healthdb.KVEntry{}).Where("module = ? AND cached_at >= ?", "alarm", sinceTime).Count(&cnt).Error
	} else {
		_ = db.Model(&healthdb.KVEntry{}).Where("module = ?", "alarm").Count(&cnt).Error
	}
	return int(cnt)
}

func Main(cmd *cobra.Command, args []string) {
	common.ScriptName = "sshNotifier"

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "main").
		Str("action", "initialization").
		Msg("Starting SSH notifier initialization")

	common.Init()

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "main").
		Str("action", "common_init_completed").
		Msg("Common initialization completed")

	viper.SetDefault("webhook.stream", "ssh")
	// TODO(migration): Remove old default after migration
	viper.SetDefault("skipped_directories", []string{})
	// New keys
	viper.SetDefault("disable_db_monitoring", false)
	viper.SetDefault("skipped_components", []string{})

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "main").
		Str("action", "viper_defaults_set").
		Str("default_webhook_stream", "ssh").
		Msg("Viper defaults configured")

	common.ConfInit("ssh-notifier", &SSHNotifierConfig)

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "main").
		Str("action", "configuration_loaded").
		Str("server_os_type", SSHNotifierConfig.Server.Os_Type).
		Str("server_address", SSHNotifierConfig.Server.Address).
		Str("primary_post_url", SSHNotifierConfig.Ssh_Post_Url).
		Str("backup_post_url", SSHNotifierConfig.Ssh_Post_Url_Backup).
		Str("webhook_stream", SSHNotifierConfig.Webhook.Stream).
		// TODO(migration): Drop file_monitoring_disabled after migration
		Bool("file_monitoring_disabled", SSHNotifierConfig.Disable_File_Monitoring).
		Bool("db_monitoring_disabled", SSHNotifierConfig.Disable_Db_Monitoring).
		Int("excluded_users", len(SSHNotifierConfig.Exclude.Users)).
		Int("excluded_ips", len(SSHNotifierConfig.Exclude.IPs)).
		Int("excluded_domains", len(SSHNotifierConfig.Exclude.Domains)).
		// TODO(migration): Drop skipped_directories after migration
		Int("skipped_directories", len(SSHNotifierConfig.SkippedDirectories)).
		Int("skipped_components", len(SSHNotifierConfig.SkippedComponents)).
		Msg("SSH notifier configuration loaded successfully")

	loginInfo := GetLoginInfo("")

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "main").
		Str("action", "login_info_collected").
		Str("event_type", loginInfo.EventType).
		Str("username", loginInfo.Username).
		Str("remote_ip", loginInfo.RemoteIp).
		Str("login_method", loginInfo.LoginMethod).
		Bool("has_fingerprint", loginInfo.Fingerprint != "").
		Msg("Login information collected")

	NotifyAndSave(loginInfo)

	log.Debug().
		Str("component", "sshNotifier").
		Str("operation", "main").
		Str("action", "completed").
		Msg("SSH notifier process completed successfully")
}

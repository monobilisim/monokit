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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
)

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
		Modify_Stream bool
		Stream        string
	}
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
	scanner := bufio.NewScanner(strings.NewReader(contents))
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), pattern) {
			return scanner.Text()
		}
	}
	return ""
}

func getSHA256Fingerprint(rawKey string) string {
	// Parse the public key
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(rawKey))
	if err != nil {
		common.LogError("Error parsing key: " + err.Error())
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
	return "SHA256:" + b64Hash
}

func getFingerprintFromAuthLog(ppid string) string {
	// Check if /var/log/secure exists
	var logFile string
	if _, err := os.Stat("/var/log/secure"); os.IsNotExist(err) {
		logFile = "/var/log/auth.log"
		common.LogDebug("Using auth.log as log file")
	} else {
		logFile = "/var/log/secure"
		common.LogDebug("Using secure as log file")
	}

	keyword := "Accepted publickey"
	common.LogDebug("Using keyword: " + keyword)

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		common.LogError("Logfile " + logFile + " does not exist, aborting.")
		return ""
	}

	// Read the log file
	file, err := os.ReadFile(logFile)
	if err != nil {
		common.LogError("Error opening file: " + err.Error())
		return ""
	}

	// Extract fingerprint from log file similar to: grep "$keyword" "$logfile" | grep $PPID | tail -n 1 | awk '{print $NF}'
	lines := strings.Split(string(file), "\n")
	var matchingLines []string

	// Find lines containing both keyword and PPID
	for _, line := range lines {
		if strings.Contains(line, keyword) && strings.Contains(line, ppid) {
			matchingLines = append(matchingLines, line)
		}
	}

	// Get the last matching line if any found
	if len(matchingLines) > 0 {
		lastLine := matchingLines[len(matchingLines)-1]
		// Extract the last field (equivalent to awk '{print $NF}')
		fields := strings.Fields(lastLine)
		if len(fields) > 0 {
			fingerprint := fields[len(fields)-1]
			common.LogDebug("Extracted fingerprint from log file: " + fingerprint)
			return fingerprint
		}
	}

	return ""
}

func GetLoginInfo(customType string) LoginInfoOutput {
	var eventType string

	// If no eventType provided, determine it from PAM_TYPE environment variable
	if customType == "" {
		eventType = os.Getenv("PAM_TYPE")
		common.LogDebug("PAM_TYPE from environment: " + customType)
	} else {
		eventType = customType
	}

	// Ensure eventType is set to a default if it's still empty
	if eventType == "" {
		common.LogDebug("Event type is empty, defaulting to 'unknown'")
		eventType = "unknown"
	}

	common.LogDebug("Getting login info with event type: " + eventType)

	var loginMethod string
	var fingerprint string
	var ppid string
	var authorizedKeys string
	var username string

	ppid = strconv.Itoa(os.Getppid())
	common.LogDebug("PPID: " + ppid)

	// First try to get fingerprint from SSH_AUTH_INFO_0 environment variable
	sshAuthInfo := os.Getenv("SSH_AUTH_INFO_0")
	if sshAuthInfo != "" {
		// Trim any whitespace and newlines
		sshAuthInfo = strings.TrimSpace(sshAuthInfo)
		common.LogDebug("Found SSH_AUTH_INFO_0: " + sshAuthInfo)
		// SSH_AUTH_INFO_0 format: "publickey ssh-ed25519 key"
		fields := strings.Fields(sshAuthInfo)
		if len(fields) >= 3 {
			fingerprint = getSHA256Fingerprint(sshAuthInfo) // Pass the full key string
			common.LogDebug("Converted raw key to SHA256 fingerprint: " + fingerprint)
		}
	}

	// If no fingerprint from SSH_AUTH_INFO_0, try auth.log
	if fingerprint == "" {
		fingerprint = getFingerprintFromAuthLog(ppid)
	}

	if fingerprint == "" {
		common.LogDebug("No fingerprint found, trying to match by PAM_USER")
	}

	pamUser := os.Getenv("PAM_USER")
	common.LogDebug("PAM_USER: " + pamUser)

	if pamUser == "root" {
		authorizedKeys = "/root/.ssh/authorized_keys"
	} else {
		authorizedKeys = "/home/" + pamUser + "/.ssh/authorized_keys"
	}
	common.LogDebug("Looking for authorized keys in: " + authorizedKeys)

	// Try first to determine if this is an SSH key login by checking logs for fingerprint
	var sshKeyLogin bool = false

	if _, err := os.Stat(authorizedKeys); err == nil {
		if SSHNotifierConfig.Server.Os_Type == "GENERIC" {
			keysOut, err := exec.Command("/usr/bin/ssh-keygen", "-lf", authorizedKeys).Output()
			if err != nil {
				common.LogError("Error getting keys: " + err.Error())
				return LoginInfoOutput{}
			}
			common.LogDebug("ssh-keygen output: " + string(keysOut))

			// Split output into lines
			keysOutSplit := strings.Split(string(keysOut), "\n")

			// Check each line for fingerprint match
			if fingerprint != "" {
				common.LogDebug("Searching for fingerprint: " + fingerprint)
				for _, key := range keysOutSplit {
					if len(key) == 0 {
						continue
					}

					if strings.Contains(key, fingerprint) {
						common.LogDebug("Found key with matching fingerprint: " + key)
						// Extract the third field (username)
						fields := strings.Fields(key)
						common.LogDebug("Fields: " + fmt.Sprintf("%+v", fields))
						if len(fields) >= 3 {
							username = fields[2]
							common.LogDebug("Username: " + username)
							loginMethod = "ssh-key"
							sshKeyLogin = true
							common.LogDebug("Found key with matching fingerprint, username: " + username)
							break
						}
					}
				}
			}
		}
	} else {
		username = pamUser
	}

	var userTmp string

	for _, excludeUser := range SSHNotifierConfig.Exclude.Users {
		userTmp = username
		if userTmp == "" {
			userTmp = pamUser
		}

		if strings.Contains(userTmp, "@") {
			userTmp = strings.Split(userTmp, "@")[0]
		}

		if userTmp == excludeUser {
			common.LogDebug("Excluding user: " + excludeUser)
			return LoginInfoOutput{}
		}
	}

	for _, excludeIp := range SSHNotifierConfig.Exclude.IPs {
		if os.Getenv("PAM_RHOST") == excludeIp && os.Getenv("PAM_RHOST") != "" {
			common.LogDebug("Excluding IP: " + os.Getenv("PAM_RHOST"))
			return LoginInfoOutput{}
		}
	}

	for _, excludeDomain := range SSHNotifierConfig.Exclude.Domains {
		if strings.Contains(userTmp, excludeDomain) && userTmp != "" {
			common.LogDebug("Excluding domain: " + excludeDomain)
			return LoginInfoOutput{}
		}
	}

	// Set login method based on our determination
	if sshKeyLogin {
		loginMethod = "ssh-key"
		common.LogDebug("Login method set to: " + loginMethod)
	} else {
		loginMethod = "password"
		common.LogDebug("Login method set to: " + loginMethod)
	}

	common.LogDebug("Final event type: " + eventType)

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

	// Double check the struct value before returning
	common.LogDebug("Full struct: " + fmt.Sprintf("%+v", result))

	return result
}

func listFiles(dir string) []string {
	// Check if the directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []string{}
	}

	var files []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && (filepath.Ext(path) == ".log") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		common.LogError("Error walking the path: " + err.Error())
	}

	return files
}

func PostToDb(postUrl string, dbReq DatabaseRequest) error {
	common.LogDebug("Posting to database URL: " + postUrl)
	// Marshal the struct to JSON
	jsonReq, err := json.Marshal(dbReq)
	if err != nil {
		common.LogDebug("Error marshaling request: " + err.Error())
		return err
	}

	req, err := http.NewRequest("POST", postUrl, bytes.NewBuffer(jsonReq))
	if err != nil {
		common.LogDebug("Error creating request: " + err.Error())
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		common.LogDebug("Error making request: " + err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		common.LogDebug("Received non-200 status code: " + strconv.Itoa(resp.StatusCode))
		return err
	}

	common.LogDebug("Successfully posted to database")
	return nil
}

func NotifyAndSave(loginInfo LoginInfoOutput) {
	if loginInfo.EventType == "" {
		common.LogDebug("Event type is empty, skipping notification and save process")
		return
	}

	common.LogDebug("Starting notification and save process")
	common.LogDebug("Event type received: " + loginInfo.EventType)

	var message string

	if loginInfo.EventType == "open_session" {
		message = "[ " + common.Config.Identifier + " ] " + "[ :green_circle: Login ] { " + loginInfo.Username + "@" + loginInfo.RemoteIp + " } >> { " + SSHNotifierConfig.Server.Address + " - " + loginInfo.Ppid + " }"
		common.LogDebug("Processing login event")
	} else {
		message = "[ " + common.Config.Identifier + " ] " + "[ :red_circle: Logout ] { " + loginInfo.Username + "@" + loginInfo.RemoteIp + " } << { " + SSHNotifierConfig.Server.Address + " - " + loginInfo.Ppid + " }"
		common.LogDebug("Processing logout event")
	}

	if strings.Contains(loginInfo.Username, "@") {
		loginInfo.Username = strings.Split(loginInfo.Username, "@")[0]
		common.LogDebug("Cleaned username: " + loginInfo.Username)
	}

	fileList := slices.Concat(listFiles("/tmp/mono"), listFiles("/tmp/mono.sh"))
	common.LogDebug("Found " + strconv.Itoa(len(fileList)) + " files in monitoring directories")

	if len(fileList) == 0 {
		common.LogDebug("No files found, using webhook configuration")
		if !SSHNotifierConfig.Webhook.Modify_Stream {
			common.Alarm(message, "", "", false)
		} else {
			var usernameOnStream string
			if strings.Contains(loginInfo.Username, "@") {
				usernameOnStream = strings.Split(loginInfo.Username, "@")[0]
			} else {
				usernameOnStream = loginInfo.Username
			}

			common.Alarm(message, SSHNotifierConfig.Webhook.Stream, usernameOnStream, true)
		}
	} else {
		common.LogDebug("Files found, sending standard alarm")
		common.Alarm(message, "", "", false)
	}

	var dbReq DatabaseRequest
	common.LogDebug("Preparing database request")

	dbReq.Ppid = "'" + loginInfo.Ppid + "'"
	dbReq.LinuxUser = "'" + loginInfo.PamUser + "'"
	dbReq.Type = "'" + loginInfo.EventType + "'"
	dbReq.KeyComment = "'" + loginInfo.Username + "'"
	dbReq.Host = "'" + loginInfo.Server + "'"
	dbReq.ConnectedFrom = "'" + loginInfo.RemoteIp + "'"
	dbReq.LoginType = "'" + loginInfo.LoginMethod + "'"

	err := PostToDb(SSHNotifierConfig.Ssh_Post_Url, dbReq)
	if err != nil {
		common.LogDebug("Primary database post failed, trying backup URL")
		err = PostToDb(SSHNotifierConfig.Ssh_Post_Url_Backup, dbReq)
		if err != nil {
			common.LogError("Error posting to db: " + err.Error())
		}
	}
}

func Main(cmd *cobra.Command, args []string) {
	common.ScriptName = "sshNotifier"
	common.Init()
	common.LogDebug("Initializing SSH notifier")

	viper.SetDefault("webhook.modify_stream", true)
	viper.SetDefault("webhook.stream", "ssh")
	common.ConfInit("ssh-notifier", &SSHNotifierConfig)
	common.LogDebug("Configuration loaded")

	NotifyAndSave(GetLoginInfo(""))
	common.LogDebug("SSH notifier process completed")
}

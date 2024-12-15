package sshNotifier

import (
    "os"
    "time"
	"io/fs"
    "bufio"
	"bytes"
	"slices"
	"os/exec"
	"strconv"
    "strings"
	"net/http"
	"io/ioutil"
	"path/filepath"
	"encoding/json"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
    "github.com/monobilisim/monokit/common"
)

var SSHNotifierConfig struct {
    Exclude struct {
		Domains []string
    	IPs []string
    	Users []string
	}

    Server struct {
        Os_Type string
        Address string
    }

    Ssh_Post_Url string
    Ssh_Post_Url_Backup string

    Webhook struct {
        Modify_Stream bool
        Stream string
    }
}

type LoginInfoOutput struct {
    Username string `json:"username"`
    Fingerprint string `json:"fingerprint"`
    Server string `json:"server"`
    RemoteIp string `json:"remote_ip"`
    Date string `json:"date"`
    Type string `json:"type"`
    LoginMethod string `json:"login_method"`
	Ppid string `json:"ppid"`
	PamUser string `json:"pam_user"`
}

type DatabaseRequest struct {
	Ppid string `json:"PPID"`
	LinuxUser string `json:"linux_user"`
	Type string `json:"type"`
	KeyComment string `json:"key_comment"`
	Host string `json:"host"`
	ConnectedFrom string `json:"connected_from"`
	LoginType string `json:"login_type"`
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

func GetLoginInfo(customType string) LoginInfoOutput {
    var logFile string
	var loginMethod string
    var keyword string
    var fingerprint string
    var ppid string
    var authorizedKeys string
	var username string

    ppid = strconv.Itoa(os.Getppid())

    // Check if /var/log/secure exists
    if _, err := os.Stat("/var/log/secure"); os.IsNotExist(err) {
        logFile = "/var/log/auth.log"
    } else {
        logFile = "/var/log/secure"
    }

    if SSHNotifierConfig.Server.Os_Type == "RHEL6" {
        keyword = "Found matching"
    } else {
        keyword = "Accepted publickey"
    }

    if _, err := os.Stat(logFile); os.IsNotExist(err) {
        common.LogError("Logfile " + logFile + " does not exist, aborting.")
        return LoginInfoOutput{}
    }

    // Read the log file
    file, err := ioutil.ReadFile(logFile)
    if err != nil {
        common.LogError("Error opening file: " + err.Error())
        return LoginInfoOutput{}
    }

	fileArray := strings.Split(string(file), "\n")


    for i := len(fileArray)-1; i >= 0; i-- {
        // Check if the line contains the keyword
        if strings.Contains(fileArray[i], keyword) {
            // Check if the line contains the PPID
            if strings.Contains(fileArray[i], ppid) {
                // Get the fingerprint, split the line and get the last part
				// buggy atm: todo fix
				tmp := strings.Split(Grep(ppid, fileArray[i]), "\n")
				tmp = strings.Split(tmp[len(tmp)-1], " ")
				fingerprint = tmp[len(tmp)-1]
                break
            }
        }
    }
    
    pamUser := os.Getenv("PAM_USER")

    if pamUser == "root" {
        authorizedKeys = "/root/.ssh/authorized_keys"
    } else {
        authorizedKeys = "/home/" + pamUser + "/.ssh/authorized_keys"
    }

    if _, err := os.Stat(authorizedKeys); err == nil {
        if SSHNotifierConfig.Server.Os_Type == "RHEL6" {
			sshKeysCmdOut, _ := os.ReadFile(authorizedKeys)
			sshKeys := strings.Split(string(sshKeysCmdOut), "\n")
            
            var comment string

			for _, key := range sshKeys {
                comment_multi := strings.Split(key, " ")
                
                if len(comment_multi) >= 2 {
                    comment = comment_multi[2]
                } else {  
                    comment = ""
                }

				if comment == "" {
					comment = "empty_comment"
				}
				common.WriteToFile(key, "/tmp/ssh_keys/" + comment)
			}


			items, _ := ioutil.ReadDir("/tmp/ssh_keys")
    		for _, item := range items {
				// Run ssh-keygen -lf on the key
				keysOut, err := exec.Command("/usr/bin/ssh-keygen", "-lf", "/tmp/ssh_keys/" + item.Name()).Output()
					
				if err != nil {
					common.LogError("Error getting keys: " + err.Error())
					return LoginInfoOutput{}
				}

				if fingerprint != "" && strings.Contains(string(keysOut), fingerprint) { 
					username = item.Name()
					loginMethod = "ssh-key"
					break
				}
			}

            if username == "" {
                username = pamUser
            }
			
			// Remove directory
			os.RemoveAll("/tmp/ssh_keys")
        } else if SSHNotifierConfig.Server.Os_Type == "GENERIC" {
            keysOut, err := exec.Command("/usr/bin/ssh-keygen", "-lf", authorizedKeys).Output()
            if err != nil {
				common.LogError("Error getting keys: " + err.Error())
                return LoginInfoOutput{}
            }
            keysOutSplit := strings.Split(string(keysOut), "\n")
            for _, key := range keysOutSplit {
                if fingerprint != "" && strings.Contains(key, fingerprint) {
                    username = strings.Split(key, " ")[2]
                    loginMethod = "ssh-key"
                    break
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
			return LoginInfoOutput{}
		}
	}

	for _, excludeIp := range SSHNotifierConfig.Exclude.IPs {
		if os.Getenv("PAM_RHOST") == excludeIp && os.Getenv("PAM_RHOST") != "" {
			return LoginInfoOutput{}
		}
	}

	for _, excludeDomain := range SSHNotifierConfig.Exclude.Domains {
		if strings.Contains(userTmp, excludeDomain) && userTmp != "" {
			return LoginInfoOutput{}
		}
	}

	if loginMethod == "" {
		loginMethod = "password"
	}

	var pamType string
	if customType != "" {
		pamType = customType
	} else {
		pamType = os.Getenv("PAM_TYPE")
	}

	return LoginInfoOutput{
		Username: username,
		Fingerprint: fingerprint,
		Server: pamUser + "@" + common.Config.Identifier,
		RemoteIp: os.Getenv("PAM_RHOST"),
		Date: time.Now().Format("02.01.2006 15:04:05"),
		Type: pamType,
		LoginMethod: loginMethod,
		Ppid: ppid,
		PamUser: pamUser,
	}

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
	// Marshal the struct to JSON
	jsonReq, err := json.Marshal(dbReq)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", postUrl, bytes.NewBuffer(jsonReq))
	if err != nil  {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{
		Timeout: time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return err
	}

	return nil
}

func NotifyAndSave(loginInfo LoginInfoOutput) {
	var message string

	if loginInfo.Type == "open_session" {
		message = "[ " + common.Config.Identifier + " ] " + "[ :green: Login ] { " + loginInfo.Username + "@" + loginInfo.RemoteIp + " } >> { " + SSHNotifierConfig.Server.Address + " - " + loginInfo.Ppid + " }"
	} else {
		message = "[ " + common.Config.Identifier + " ] " + "[ :red_circle: Logout ] { " + loginInfo.Username + "@" + loginInfo.RemoteIp + " } << { " + SSHNotifierConfig.Server.Address + " - " + loginInfo.Ppid + " }"
	}
	
	if strings.Contains(loginInfo.Username, "@") {
		loginInfo.Username = strings.Split(loginInfo.Username, "@")[0]
	}

	fileList := slices.Concat(listFiles("/tmp/mono"), listFiles("/tmp/mono.sh"))

	if len(fileList) == 0 {
        if !SSHNotifierConfig.Webhook.Modify_Stream {
            common.Alarm(message, "", "", false)
        } else {
		    common.Alarm(message, SSHNotifierConfig.Webhook.Stream, loginInfo.Username, true)
        }
	} else {
		common.Alarm(message, "", "", false)
	}

	var dbReq DatabaseRequest

	dbReq.Ppid = "'" + loginInfo.Ppid + "'"
	dbReq.LinuxUser = "'" + loginInfo.PamUser + "'"
	dbReq.Type = "'" + loginInfo.Type + "'"
	dbReq.KeyComment = "'" + loginInfo.Username + "'"
	dbReq.Host = "'" + loginInfo.Server + "'"
	dbReq.ConnectedFrom = "'" + loginInfo.RemoteIp + "'"
	dbReq.LoginType = "'" + loginInfo.LoginMethod + "'"

	err := PostToDb(SSHNotifierConfig.Ssh_Post_Url, dbReq)
	if err != nil {
		err = PostToDb(SSHNotifierConfig.Ssh_Post_Url_Backup, dbReq)
		if err != nil {
			common.LogError("Error posting to db: " + err.Error())
		}
	}
}
        
func Main(cmd *cobra.Command, args []string) {
    common.ScriptName = "sshNotifier"
    common.Init()
    viper.SetDefault("webhook.modify_stream", true)
    viper.SetDefault("webhook.stream", "ssh")
    common.ConfInit("ssh-notifier", &SSHNotifierConfig)

	var customType string
	login, _ := cmd.Flags().GetBool("login")
	logout, _ := cmd.Flags().GetBool("logout")

	if login {
		customType = "open_session"
	} else if logout {
		customType = "close_session"
	}

    time.Sleep(1 * time.Second) // Wait for PAM to finish

	NotifyAndSave(GetLoginInfo(customType))
}

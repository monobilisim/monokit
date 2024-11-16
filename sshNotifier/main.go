package sshNotifier

import (
    "os"
    "fmt"
    "time"
	"io/fs"
    "bufio"
	"bytes"
	"slices"
	"os/exec"
    "strings"
	"net/http"
	"io/ioutil"
	"path/filepath"
	"encoding/json"
    "github.com/spf13/cobra"
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

func GetLoginInfo() LoginInfoOutput {
    var logFile string
	var loginMethod string
    var keyword string
    var fingerprint string
    var ppid string
    var authorizedKeys string
	var username string

    ppid = os.Getenv("PPID")

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
        fmt.Println("Logfile " + logFile + " does not exist, aborting.")
        return LoginInfoOutput{}
    }

    // Read the log file
    file, err := ioutil.ReadFile(logFile)
    if err != nil {
        fmt.Println("Error opening file:", err)
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

			for _, key := range sshKeys {
				comment := strings.Split(key, " ")[2]
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

	return LoginInfoOutput{
		Username: username,
		Fingerprint: fingerprint,
		Server: pamUser + "@" + common.Config.Identifier,
		RemoteIp: os.Getenv("PAM_RHOST"),
		Date: time.Now().Format("02.01.2006 15:04:05"),
		Type: os.Getenv("PAM_TYPE"),
		LoginMethod: loginMethod,
	}

}

func listFiles(dir string) []string {
    var files []string

    err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
       if !d.IsDir() && filepath.Ext(path) == ".log" && filepath.Ext(path) == ".txt" {
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
		message = "[ " + common.Config.Identifier + " ] " + "[ :green: Login ] { " + loginInfo.Username + "@" + loginInfo.RemoteIp + " } >> " + loginInfo.Server + " - " + os.Getenv("PPID") + " }"
	} else {
		message = "[ " + common.Config.Identifier + " ] " + "[ :red_circle: Logout ] { " + loginInfo.Username + "@" + loginInfo.RemoteIp + " } << " + loginInfo.Server + " - " + os.Getenv("PPID") + " }"
	}
	
	if strings.Contains(loginInfo.Username, "@") {
		loginInfo.Username = strings.Split(loginInfo.Username, "@")[0]
	}

	fileList := slices.Concat(listFiles("/tmp/mono"), listFiles("/tmp/mono.sh"))

	if len(fileList) > 0 {
		common.Alarm(message, "", "", false)
	} else {
		common.Alarm(message, SSHNotifierConfig.Webhook.Stream, loginInfo.Username, true)
	}

	var dbReq DatabaseRequest

	dbReq.Ppid = "'" + os.Getenv("PPID") + "'"
	dbReq.LinuxUser = "'" + os.Getenv("PAM_USER") + "'"
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
    common.ConfInit("ssh-notifier", &SSHNotifierConfig)

	//login, _ := cmd.Flags().GetBool("login")
	//logout, _ := cmd.Flags().GetBool("logout")

    time.Sleep(1 * time.Second) // Wait for PAM to finish

	NotifyAndSave(GetLoginInfo())
}

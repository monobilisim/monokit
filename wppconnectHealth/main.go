package wppconnectHealth

import (
	"fmt"
	"github.com/monobilisim/monokit/common"
	"github.com/spf13/cobra"
	"net/http"
	"os"
    "encoding/json"
	"time"
)

var Config struct {
	Wpp struct {
        Secret string
        Url string
    }
}

func GetStatus(session string, token string) string {
    // Authorization: Bearer token
    client := &http.Client{}
    req, err := http.NewRequest("GET", Config.Wpp.Url + "/api/" + session + "/check-connection-session", nil)
    if err != nil {
        common.LogError("Error while checking connection: " + err.Error())
        os.Exit(1)
    }

    req.Header.Add("Authorization", "Bearer " + token)
    resp, err := client.Do(req)
    if err != nil {
        common.LogError("Error while checking connection: " + err.Error())
        os.Exit(1)
    }

    defer resp.Body.Close()

    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    status := result["message"].(string)

    return status
}

func GetContactName(session string, token string) string {
    // Authorization: Bearer
    client := &http.Client{}
    req, err := http.NewRequest("GET", Config.Wpp.Url + "/api/" + session + "/contact/" + session, nil)

    if err != nil {
        common.LogError("Error while getting contact name: " + err.Error())
        os.Exit(1)
    }

    req.Header.Add("Authorization", "Bearer " + token)
    resp, err := client.Do(req)
    if err != nil {
        common.LogError("Error while getting contact name: " + err.Error())
        os.Exit(1)
    }

    defer resp.Body.Close()

    var result map[string]interface{}
    
    json.NewDecoder(resp.Body).Decode(&result)

    var contactName string

    if result["response"].(map[string]interface{})["name"] != nil {
        contactName = result["response"].(map[string]interface{})["name"].(string)
    }

    
    if contactName == "" {
        if result["response"].(map[string]interface{})["pushname"] != nil {
            contactName = result["response"].(map[string]interface{})["pushname"].(string)
        }
        if contactName == "" {
            contactName = "No Name"
        }
    }

    return contactName
}



func GetToken(session string) string {
    client := &http.Client{}
    req, err := http.NewRequest("POST", Config.Wpp.Url + "/api/" + session + "/" + Config.Wpp.Secret + "/generate-token", nil)
    if err != nil {
        common.LogError("Error while generating token: " + err.Error())
        os.Exit(1)
    }

    req.Header.Add("Content-Type", "application/json")
    resp, err := client.Do(req)
    if err != nil {
        common.LogError("Error while generating token: " + err.Error())
        os.Exit(1)
    }

    defer resp.Body.Close()
    
    var token map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&token)
    tokenStr := token["token"].(string)
    return tokenStr
}

func WppCheck() {
    // GET request to Config.Wpp.Url + "/api/" + Config.Wpp.Secret + "/show-all-sessions"
    url := Config.Wpp.Url + "/api/" + Config.Wpp.Secret + "/show-all-sessions"
    resp, err := http.Get(url)
    if err != nil {
        common.LogError("Error while getting sessions: " + err.Error())
        os.Exit(1)
    }
    defer resp.Body.Close()

    // Check if the response is 200
    if resp.StatusCode != 200 {
        common.LogError("Error while getting sessions: Status " + resp.Status)
        os.Exit(1)
    }

    // Read the response
    var result map[string]interface{}
    json.NewDecoder(resp.Body).Decode(&result)
    // result["response"] is an array of sessions
    sessions := result["response"].([]interface{})

    for _, session := range sessions {
        token := GetToken(session.(string))
        status := GetStatus(session.(string), token)
        contactName := GetContactName(session.(string), token)

        if status == "Connected" {
            fmt.Println(common.Blue + contactName + ", Session " + session.(string) + " " + common.Green + status + common.Reset)
            common.AlarmCheckUp(session.(string), "Session " + session.(string) + ", named '" + contactName + "', is now " + status, false)
        } else {
            fmt.Println(common.Blue + contactName + ", Session " + session.(string) + " " + common.Fail + status + common.Reset)
            common.AlarmCheckDown(session.(string), "Session " + session.(string) + ", named '" + contactName + "', is " + status, false)
        }
    }
}



func Main(cmd *cobra.Command, args []string) {
	version := "2.0.0"
	common.ScriptName = "wppconnectHealth"
	common.TmpDir = common.TmpDir + "Health"
	common.Init()
    common.ConfInit("wppconnect", &Config)

	fmt.Println("WPPConnect Health REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05") + "\n")
    
    WppCheck()

}

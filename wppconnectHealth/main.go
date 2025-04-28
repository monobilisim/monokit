package wppconnectHealth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/monobilisim/monokit/common"
	api "github.com/monobilisim/monokit/common/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper" // Import viper for config reading in detection
)

// DetectWppconnect checks if the WPPConnect service seems to be configured and reachable.
func DetectWppconnect() bool {
	// 1. Try to load the configuration
	var tempConfig struct {
		Wpp struct {
			Secret string
			Url    string
		}
	}
	// Use viper directly to avoid initializing the full common stack just for detection
	viper.SetConfigName("wppconnect")
	viper.AddConfigPath("/etc/mono") // Assuming standard config path
	err := viper.ReadInConfig()
	if err != nil {
		common.LogDebug(fmt.Sprintf("wppconnectHealth auto-detection failed: Cannot read config file: %v", err))
		return false
	}
	err = viper.Unmarshal(&tempConfig)
	if err != nil {
		common.LogDebug(fmt.Sprintf("wppconnectHealth auto-detection failed: Cannot unmarshal config: %v", err))
		return false
	}

	// 2. Check if essential config values are present
	if tempConfig.Wpp.Url == "" || tempConfig.Wpp.Secret == "" {
		common.LogDebug("wppconnectHealth auto-detection failed: Missing Wpp.Url or Wpp.Secret in config.")
		return false
	}
	common.LogDebug("wppconnectHealth auto-detection: Found URL and Secret in config.")

	// 3. Attempt to reach the API endpoint
	url := tempConfig.Wpp.Url + "/api/" + tempConfig.Wpp.Secret + "/show-all-sessions"
	client := &http.Client{Timeout: 5 * time.Second} // Use a short timeout for detection
	resp, err := client.Get(url)
	if err != nil {
		common.LogDebug(fmt.Sprintf("wppconnectHealth auto-detection failed: Error contacting API endpoint %s: %v", url, err))
		return false
	}
	defer resp.Body.Close()

	// Check for a successful status code (e.g., 200 OK)
	// Other statuses might indicate issues, but reachability is the primary goal here.
	if resp.StatusCode != http.StatusOK {
		common.LogDebug(fmt.Sprintf("wppconnectHealth auto-detection failed: API endpoint %s returned status %s", url, resp.Status))
		return false
	}

	common.LogDebug(fmt.Sprintf("wppconnectHealth auto-detection: Successfully contacted API endpoint %s.", url))
	common.LogDebug("wppconnectHealth auto-detected successfully.")
	return true
}

func init() {
	common.RegisterComponent(common.Component{
		Name:       "wppconnectHealth",
		EntryPoint: Main,
		Platform:   "any",            // Interacts via HTTP API, platform-agnostic
		AutoDetect: DetectWppconnect, // Add the auto-detect function
	})
}

var Config struct {
	Wpp struct {
		Secret string
		Url    string
	}
}

func GetStatus(session string, token string) string {
	// Authorization: Bearer token
	client := &http.Client{}
	req, err := http.NewRequest("GET", Config.Wpp.Url+"/api/"+session+"/check-connection-session", nil)
	if err != nil {
		common.LogError("Error while checking connection: " + err.Error())
		os.Exit(1)
	}

	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		common.LogError("Error while checking connection: " + err.Error())
		os.Exit(1)
	}

	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	// Handle potential nil map or missing key gracefully
	message := "Unknown"
	if msg, ok := result["message"].(string); ok {
		message = msg
	} else {
		common.LogDebug(fmt.Sprintf("wppconnectHealth: Unexpected response format or missing 'message' key for session %s status check.", session))
	}

	return message
}

func GetContactName(session string, token string) string {
	// Authorization: Bearer
	client := &http.Client{}
	req, err := http.NewRequest("GET", Config.Wpp.Url+"/api/"+session+"/contact/"+session, nil)

	if err != nil {
		common.LogError("Error while getting contact name: " + err.Error())
		os.Exit(1)
	}

	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		common.LogError("Error while getting contact name: " + err.Error())
		os.Exit(1)
	}

	defer resp.Body.Close()

	var result map[string]interface{}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		common.LogError(fmt.Sprintf("Error decoding contact name response for session %s: %v", session, err))
		return "ErrorDecoding"
	}

	var contactName string

	// More robust checking for nested map structure and keys
	if responseMap, ok := result["response"].(map[string]interface{}); ok {
		if name, ok := responseMap["name"].(string); ok && name != "" {
			contactName = name
		} else if pushname, ok := responseMap["pushname"].(string); ok && pushname != "" {
			contactName = pushname
		}
	}

	if contactName == "" {
		contactName = "No Name"
	}

	return contactName
}

func GetToken(session string) string {
	client := &http.Client{}
	req, err := http.NewRequest("POST", Config.Wpp.Url+"/api/"+session+"/"+Config.Wpp.Secret+"/generate-token", nil)
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
	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		common.LogError(fmt.Sprintf("Error decoding token response for session %s: %v", session, err))
		return "" // Return empty string on error
	}

	tokenStr := ""
	if tok, ok := token["token"].(string); ok {
		tokenStr = tok
	} else {
		common.LogError(fmt.Sprintf("wppconnectHealth: Unexpected response format or missing 'token' key for session %s token generation.", session))
	}
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
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		common.LogError(fmt.Sprintf("Error decoding show-all-sessions response: %v", err))
		os.Exit(1)
	}

	// Check if "response" key exists and is an array
	sessionsInterface, ok := result["response"]
	if !ok {
		common.LogError("wppconnectHealth: Missing 'response' key in show-all-sessions result.")
		os.Exit(1)
	}
	sessions, ok := sessionsInterface.([]interface{})
	if !ok {
		common.LogError("wppconnectHealth: 'response' key is not an array in show-all-sessions result.")
		os.Exit(1)
	}

	for _, sessionInterface := range sessions {
		session, ok := sessionInterface.(string)
		if !ok {
			common.LogDebug(fmt.Sprintf("wppconnectHealth: Skipping non-string session identifier: %v", sessionInterface))
			continue
		}

		token := GetToken(session)
		if token == "" {
			common.LogError(fmt.Sprintf("wppconnectHealth: Could not get token for session %s, skipping status check.", session))
			continue // Skip if token generation failed
		}
		status := GetStatus(session, token)
		contactName := GetContactName(session, token)

		if status == "Connected" {
			fmt.Println(common.Blue + contactName + ", Session " + session + " " + common.Green + status + common.Reset)
			common.AlarmCheckUp(session, "Session "+session+", named '"+contactName+"', is now "+status, false)
		} else {
			fmt.Println(common.Blue + contactName + ", Session " + session + " " + common.Fail + status + common.Reset)
			common.AlarmCheckDown(session, "Session "+session+", named '"+contactName+"', is "+status, false, "", "")
		}
	}
}

func Main(cmd *cobra.Command, args []string) {
	version := "2.0.0"
	common.ScriptName = "wppconnectHealth"
	common.TmpDir = common.TmpDir + "Health" // Note: This might collide with other health checks if not unique
	common.Init()
	// Load config here, after common.Init which sets up viper paths
	common.ConfInit("wppconnect", &Config)

	// Check essential config after loading
	if Config.Wpp.Url == "" || Config.Wpp.Secret == "" {
		common.LogError("wppconnectHealth: Missing Wpp.Url or Wpp.Secret in configuration. Cannot proceed.")
		fmt.Println("Error: Missing Wpp.Url or Wpp.Secret in configuration.")
		os.Exit(1) // Exit if essential config is missing
	}

	api.WrapperGetServiceStatus("wppconnectHealth")

	fmt.Println("WPPConnect Health REWRITE - v" + version + " - " + time.Now().Format("2006-01-02 15:04:05") + "\n")

	WppCheck()

}

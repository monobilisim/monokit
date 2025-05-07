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
	client := &http.Client{}
	req, err := http.NewRequest("GET", Config.Wpp.Url+"/api/"+session+"/check-connection-session", nil)
	if err != nil {
		common.LogError("Error while checking connection: " + err.Error())
		return "Error"
	}

	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		common.LogError("Error while checking connection: " + err.Error())
		return "Error"
	}

	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	message := "Unknown"
	if msg, ok := result["message"].(string); ok {
		message = msg
	} else {
		common.LogDebug(fmt.Sprintf("wppconnectHealth: Unexpected response format or missing 'message' key for session %s status check.", session))
	}

	return message
}

func GetContactName(session string, token string) string {
	client := &http.Client{}
	req, err := http.NewRequest("GET", Config.Wpp.Url+"/api/"+session+"/contact/"+session, nil)
	if err != nil {
		common.LogError("Error while getting contact name: " + err.Error())
		return "Error"
	}

	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		common.LogError("Error while getting contact name: " + err.Error())
		return "Error"
	}

	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		common.LogError(fmt.Sprintf("Error decoding contact name response for session %s: %v", session, err))
		return "ErrorDecoding"
	}

	var contactName string
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
		return ""
	}

	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		common.LogError("Error while generating token: " + err.Error())
		return ""
	}

	defer resp.Body.Close()

	var token map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		common.LogError(fmt.Sprintf("Error decoding token response for session %s: %v", session, err))
		return ""
	}

	tokenStr := ""
	if tok, ok := token["token"].(string); ok {
		tokenStr = tok
	} else {
		common.LogError(fmt.Sprintf("wppconnectHealth: Unexpected response format or missing 'token' key for session %s token generation.", session))
	}
	return tokenStr
}

func WppCheck() *WppConnectHealthData {
	healthData := &WppConnectHealthData{
		Healthy: true,
	}

	url := Config.Wpp.Url + "/api/" + Config.Wpp.Secret + "/show-all-sessions"
	resp, err := http.Get(url)
	if err != nil {
		common.LogError("Error while getting sessions: " + err.Error())
		healthData.Healthy = false
		return healthData
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		common.LogError("Error while getting sessions: Status " + resp.Status)
		healthData.Healthy = false
		return healthData
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		common.LogError(fmt.Sprintf("Error decoding show-all-sessions response: %v", err))
		healthData.Healthy = false
		return healthData
	}

	sessionsInterface, ok := result["response"]
	if !ok {
		common.LogError("wppconnectHealth: Missing 'response' key in show-all-sessions result.")
		healthData.Healthy = false
		return healthData
	}
	sessions, ok := sessionsInterface.([]interface{})
	if !ok {
		common.LogError("wppconnectHealth: 'response' key is not an array in show-all-sessions result.")
		healthData.Healthy = false
		return healthData
	}

	healthData.TotalCount = len(sessions)
	healthData.HealthyCount = 0

	for _, sessionInterface := range sessions {
		session, ok := sessionInterface.(string)
		if !ok {
			common.LogDebug(fmt.Sprintf("wppconnectHealth: Skipping non-string session identifier: %v", sessionInterface))
			continue
		}

		token := GetToken(session)
		if token == "" {
			common.LogError(fmt.Sprintf("wppconnectHealth: Could not get token for session %s, skipping status check.", session))
			continue
		}

		status := GetStatus(session, token)
		contactName := GetContactName(session, token)
		isHealthy := status == "Connected"

		sessionData := WppConnectData{
			Session:     session,
			ContactName: contactName,
			Status:      status,
			Healthy:     isHealthy,
		}

		healthData.Sessions = append(healthData.Sessions, sessionData)

		if isHealthy {
			healthData.HealthyCount++
			common.AlarmCheckUp(session, "Session "+session+", named '"+contactName+"', is now "+status, false)
		} else {
			healthData.Healthy = false
			common.AlarmCheckDown(session, "Session "+session+", named '"+contactName+"', is "+status, false, "", "")
		}
	}

	return healthData
}

func Main(cmd *cobra.Command, args []string) {
	version := "2.0.0"
	common.ScriptName = "wppconnectHealth"
	common.TmpDir = common.TmpDir + "Health"
	common.Init()
	common.ConfInit("wppconnect", &Config)

	// Check essential config after loading
	if Config.Wpp.Url == "" || Config.Wpp.Secret == "" {
		common.LogError("wppconnectHealth: Missing Wpp.Url or Wpp.Secret in configuration. Cannot proceed.")
		fmt.Println("Error: Missing Wpp.Url or Wpp.Secret in configuration.")
		os.Exit(1)
	}

	api.WrapperGetServiceStatus("wppconnectHealth")

	// Collect health data
	healthData := WppCheck()
	healthData.Version = version

	// Render the health data using our UI system
	RenderWppConnectHealth(healthData)
}

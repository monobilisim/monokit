package wppconnectHealth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
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
		log.Debug().Err(err).Msg("wppconnectHealth auto-detection failed: Cannot read config file")
		return false
	}
	err = viper.Unmarshal(&tempConfig)
	if err != nil {
		log.Debug().Err(err).Msg("wppconnectHealth auto-detection failed: Cannot unmarshal config")
		return false
	}

	// 2. Check if essential config values are present
	if tempConfig.Wpp.Url == "" || tempConfig.Wpp.Secret == "" {
		log.Debug().Msg("wppconnectHealth auto-detection failed: Missing Wpp.Url or Wpp.Secret in config.")
		return false
	}
	log.Debug().Msg("wppconnectHealth auto-detection: Found URL and Secret in config.")

	// 3. Attempt to reach the API endpoint
	url := tempConfig.Wpp.Url + "/api/" + tempConfig.Wpp.Secret + "/show-all-sessions"
	client := &http.Client{Timeout: 5 * time.Second} // Use a short timeout for detection
	resp, err := client.Get(url)
	if err != nil {
		log.Debug().Err(err).Msg(fmt.Sprintf("wppconnectHealth auto-detection failed: Error contacting API endpoint %s", url))
		return false
	}
	defer resp.Body.Close()

	// Check for a successful status code (e.g., 200 OK)
	// Other statuses might indicate issues, but reachability is the primary goal here.
	if resp.StatusCode != http.StatusOK {
		log.Debug().Str("url", url).Str("status", resp.Status).Msg("wppconnectHealth auto-detection failed: API endpoint returned status")
		return false
	}

	log.Debug().Str("url", url).Msg("wppconnectHealth auto-detection: Successfully contacted API endpoint")
	log.Debug().Msg("wppconnectHealth auto-detected successfully.")
	return true
}

// CollectWppConnectHealthData gathers all WPPConnect health information.
func CollectWppConnectHealthData() *WppConnectHealthData {
	healthData := &WppConnectHealthData{
		Healthy: true,
		Version: "2.0.0",
	}

	url := WppConnectHealthConfig.Wpp.Url + "/api/" + WppConnectHealthConfig.Wpp.Secret + "/show-all-sessions"
	resp, err := http.Get(url)
	if err != nil {
		log.Error().Err(err).Msg("Error while getting sessions")
		healthData.Healthy = false
		return healthData
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Error().Str("status", resp.Status).Msg("Error while getting sessions")
		healthData.Healthy = false
		return healthData
	}

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Error().Err(err).Msg("Error decoding show-all-sessions response")
		healthData.Healthy = false
		return healthData
	}

	sessionsInterface, ok := result["response"]
	if !ok {
		log.Error().Msg("wppconnectHealth: Missing 'response' key in show-all-sessions result")
		healthData.Healthy = false
		return healthData
	}
	sessions, ok := sessionsInterface.([]interface{})
	if !ok {
		log.Error().Msg("wppconnectHealth: 'response' key is not an array in show-all-sessions result")
		healthData.Healthy = false
		return healthData
	}

	healthData.TotalCount = len(sessions)
	healthData.HealthyCount = 0

	for _, sessionInterface := range sessions {
		session, ok := sessionInterface.(string)
		if !ok {
			log.Debug().Interface("session", sessionInterface).Msg("wppconnectHealth: Skipping non-string session identifier")
			continue
		}

		token := GetToken(session)
		if token == "" {
			log.Error().Str("session", session).Msg("wppconnectHealth: Could not get token for session, skipping status check")
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

func GetStatus(session string, token string) string {
	client := &http.Client{}
	req, err := http.NewRequest("GET", WppConnectHealthConfig.Wpp.Url+"/api/"+session+"/check-connection-session", nil)
	if err != nil {
		log.Error().Err(err).Msg("Error while checking connection")
		return "Error"
	}

	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Error while checking connection")
		return "Error"
	}

	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	message := "Unknown"
	if msg, ok := result["message"].(string); ok {
		message = msg
	} else {
		log.Debug().Str("session", session).Msg("wppconnectHealth: Unexpected response format or missing 'message' key for session status check")
	}

	return message
}

func GetContactName(session string, token string) string {
	client := &http.Client{}
	req, err := http.NewRequest("GET", WppConnectHealthConfig.Wpp.Url+"/api/"+session+"/contact/"+session, nil)
	if err != nil {
		log.Error().Err(err).Msg("Error while getting contact name")
		return "Error"
	}

	req.Header.Add("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Error while getting contact name")
		return "Error"
	}

	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Error().Str("session", session).Err(err).Msg("Error decoding contact name response")
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
	req, err := http.NewRequest("POST", WppConnectHealthConfig.Wpp.Url+"/api/"+session+"/"+WppConnectHealthConfig.Wpp.Secret+"/generate-token", nil)
	if err != nil {
		log.Error().Err(err).Msg("Error while generating token")
		return ""
	}

	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Error while generating token")
		return ""
	}

	defer resp.Body.Close()

	var token map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		log.Error().Str("session", session).Err(err).Msg("Error decoding token response")
		return ""
	}

	tokenStr := ""
	if tok, ok := token["token"].(string); ok {
		tokenStr = tok
	} else {
		log.Error().Str("session", session).Msg("wppconnectHealth: Unexpected response format or missing 'token' key for session token generation")
	}
	return tokenStr
}

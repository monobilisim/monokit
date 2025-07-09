package common

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var TmpDir = "/tmp/mono/"
var ScriptName string

var AlarmCmd = &cobra.Command{
	Use:   "alarm",
	Short: "Alarm utilities",
}

var AlarmCheckUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Send alarm of service being up if it was down",
	Run: func(cmd *cobra.Command, args []string) {
		Init()
		service, _ := cmd.Flags().GetString("service")
		message, _ := cmd.Flags().GetString("message")
		ScriptName, _ = cmd.Flags().GetString("scriptName")
		noInterval, _ := cmd.Flags().GetBool("noInterval")
		AlarmCheckUp(service, message, noInterval)
	},
}

var AlarmCheckDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Send alarm of service being down if it was up",
	Run: func(cmd *cobra.Command, args []string) {
		Init()
		service, _ := cmd.Flags().GetString("service")
		message, _ := cmd.Flags().GetString("message")
		ScriptName, _ = cmd.Flags().GetString("scriptName")
		noInterval, _ := cmd.Flags().GetBool("noInterval")
		AlarmCheckDown(service, message, noInterval, "", "")
	},
}

var AlarmSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a plain alarm",
	Run: func(cmd *cobra.Command, args []string) {
		Init()
		message, _ := cmd.Flags().GetString("message")
		customStream, _ := cmd.Flags().GetString("stream")
		customTopic, _ := cmd.Flags().GetString("topic")
		Alarm(message, customStream, customTopic, false)
	},
}

func AlarmCheckUp(service string, message string, noInterval bool) {
	startTime := time.Now()
	// Remove slashes from service and replace them with -
	log.Debug().
		Str("component", "alarm").
		Str("action", "check_up").
		Str("service", service).
		Str("message", message).
		Bool("noInterval", noInterval).
		Msg("Starting alarm check up process")

	serviceReplaced := strings.Replace(service, "/", "-", -1)
	file_path := TmpDir + "/" + serviceReplaced + ".log"

	log.Debug().
		Str("component", "alarm").
		Str("action", "file_check").
		Str("file_path", file_path).
		Str("service_replaced", serviceReplaced).
		Msg("Processing alarm check up file")

	messageFinal := "[" + ScriptName + " - " + Config.Identifier + "] [:check:] " + message

	if _, err := os.Stat(file_path); os.IsNotExist(err) {
		log.Debug().
			Str("component", "alarm").
			Str("action", "check_up").
			Str("service", service).
			Str("file_path", file_path).
			Msg("Service state file does not exist, no alarm needed")
		return
	}

	// Open file and load the JSON
	file, err := os.OpenFile(file_path, os.O_RDONLY, 0644)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "alarm").
			Str("action", "file_open").
			Str("file_path", file_path).
			Msg("Error opening alarm state file")
		return
	}
	defer file.Close()

	var j ServiceFile

	fileRead, err := io.ReadAll(file)

	if err != nil {
		log.Error().
			Err(err).
			Str("component", "alarm").
			Str("action", "file_read").
			Str("file_path", file_path).
			Msg("Error reading alarm state file")
		return
	}

	err = json.Unmarshal(fileRead, &j)

	if err != nil {
		log.Error().
			Err(err).
			Str("component", "alarm").
			Str("action", "json_parse").
			Str("file_path", file_path).
			Str("file_content", string(fileRead)).
			Msg("Error parsing alarm state JSON")
		return
	}

	if !j.Locked && !noInterval {
		log.Debug().
			Str("component", "alarm").
			Str("action", "cleanup").
			Str("service", service).
			Str("file_path", file_path).
			Bool("was_locked", j.Locked).
			Msg("Service state not locked, removing state file")
		os.Remove(file_path)
		return
	} else {
		log.Debug().
			Str("component", "alarm").
			Str("action", "send_up_alarm").
			Str("service", service).
			Str("message", messageFinal).
			Bool("was_locked", j.Locked).
			Bool("no_interval", noInterval).
			Dur("processing_time", time.Since(startTime)).
			Msg("Sending service recovery alarm")
		os.Remove(file_path)
		Alarm(messageFinal, "", "", false)
	}
}

type ServiceFile struct {
	Date   string `json:"date"`
	Locked bool   `json:"locked"`
}

func AlarmCheckDown(service string, message string, noInterval bool, customStream string, customTopic string) {
	startTime := time.Now()
	// Remove slashes from service and replace them with -
	log.Debug().
		Str("component", "alarm").
		Str("action", "check_down").
		Str("service", service).
		Str("message", message).
		Bool("noInterval", noInterval).
		Str("customStream", customStream).
		Str("customTopic", customTopic).
		Msg("Starting alarm check down process")

	serviceReplaced := strings.Replace(service, "/", "-", -1)
	filePath := TmpDir + "/" + serviceReplaced + ".log"
	currentDate := time.Now().Format("2006-01-02 15:04:05 -0700")

	log.Debug().
		Str("component", "alarm").
		Str("action", "file_check").
		Str("filePath", filePath).
		Str("current_date", currentDate).
		Msg("Processing alarm check down file")

	messageFinal := "[" + ScriptName + " - " + Config.Identifier + "] [:red_circle:] " + message

	// Check if the file exists
	if _, err := os.Stat(filePath); err == nil && !noInterval {
		// Open file and load the JSON

		file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
		if err != nil {
			log.Error().
				Err(err).
				Str("component", "alarm").
				Str("action", "file_open").
				Str("file_path", filePath).
				Msg("Error opening alarm state file for reading")
			return
		}
		defer file.Close()

		var j ServiceFile

		fileRead, err := io.ReadAll(file)

		if err != nil {
			log.Error().
				Err(err).
				Str("component", "alarm").
				Str("action", "file_read").
				Str("file_path", filePath).
				Msg("Error reading alarm state file")
			return
		}

		err = json.Unmarshal(fileRead, &j)

		if err != nil {
			log.Error().
				Err(err).
				Str("component", "alarm").
				Str("action", "json_parse").
				Str("file_path", filePath).
				Str("file_content", string(fileRead)).
				Msg("Error parsing alarm state JSON")
			return
		}

		// Return if locked == true
		if j.Locked {
			log.Debug().
				Str("component", "alarm").
				Str("action", "check_down").
				Str("service", service).
				Bool("is_locked", j.Locked).
				Str("lock_date", j.Date).
				Msg("Service already locked, skipping alarm")
			return
		}

		oldDate := j.Date
		oldDateParsed, err := time.Parse("2006-01-02 15:04:05 -0700", oldDate)

		if err != nil {
			log.Error().
				Err(err).
				Str("component", "alarm").
				Str("action", "date_parse").
				Str("date_string", oldDate).
				Msg("Error parsing alarm state date")
		}

		finJson := &ServiceFile{
			Date:   currentDate,
			Locked: true,
		}

		if Config.Alarm.Interval == 0 {
			if oldDateParsed.Format("2006-01-02") != time.Now().Format("2006-01-02") {
				log.Debug().
					Str("component", "alarm").
					Str("action", "daily_alarm").
					Str("service", service).
					Str("old_date", oldDateParsed.Format("2006-01-02")).
					Str("current_date", time.Now().Format("2006-01-02")).
					Msg("Sending daily alarm for service down")

				jsonData, err := json.Marshal(&ServiceFile{Date: currentDate, Locked: false})

				if err != nil {
					log.Error().
						Err(err).
						Str("component", "alarm").
						Str("action", "json_marshal").
						Msg("Error marshalling alarm state JSON")
					return
				}

				err = os.WriteFile(filePath, jsonData, 0644)

				if err != nil {
					log.Error().
						Err(err).
						Str("component", "alarm").
						Str("action", "file_write").
						Str("file_path", filePath).
						Msg("Error writing alarm state file")
					return
				}

				Alarm(messageFinal, customStream, customTopic, false)
			}
			return
		}

		timeSinceHours := time.Since(oldDateParsed).Hours()
		if timeSinceHours > 24 {
			log.Debug().
				Str("component", "alarm").
				Str("action", "24h_alarm").
				Str("service", service).
				Float64("hours_since", timeSinceHours).
				Msg("Sending 24-hour interval alarm for service down")

			jsonData, err := json.Marshal(finJson)

			if err != nil {
				log.Error().
					Err(err).
					Str("component", "alarm").
					Str("action", "json_marshal").
					Msg("Error marshalling alarm state JSON")
				return
			}

			err = os.WriteFile(filePath, jsonData, 0644)

			if err != nil {
				log.Error().
					Err(err).
					Str("component", "alarm").
					Str("action", "file_write").
					Str("file_path", filePath).
					Msg("Error writing alarm state file")
				return
			}

			Alarm(messageFinal, customStream, customTopic, false)
		} else {
			if !j.Locked {
				// currentDate - oldDate in minutes
				timeDiff := time.Since(oldDateParsed) //.Minutes()

				if timeDiff.Minutes() >= Config.Alarm.Interval {
					log.Debug().
						Str("component", "alarm").
						Str("action", "interval_alarm").
						Str("service", service).
						Float64("minutes_since", timeDiff.Minutes()).
						Float64("interval_minutes", Config.Alarm.Interval).
						Msg("Sending interval-based alarm for service down")

					jsonData, err := json.Marshal(finJson)
					if err != nil {
						log.Error().
							Err(err).
							Str("component", "alarm").
							Str("action", "json_marshal").
							Msg("Error marshalling alarm state JSON")
						return
					}

					err = os.WriteFile(filePath, jsonData, 0644)

					if err != nil {
						log.Error().
							Err(err).
							Str("component", "alarm").
							Str("action", "file_write").
							Str("file_path", filePath).
							Msg("Error writing alarm state file")
						return
					}

					Alarm(messageFinal, customStream, customTopic, false)
				} else {
					log.Debug().
						Str("component", "alarm").
						Str("action", "interval_check").
						Str("service", service).
						Float64("minutes_since", timeDiff.Minutes()).
						Float64("interval_minutes", Config.Alarm.Interval).
						Msg("Service down but interval not reached yet")
				}
			}
		}
	} else {

		log.Debug().
			Str("component", "alarm").
			Str("action", "create_state").
			Str("service", service).
			Str("file_path", filePath).
			Bool("no_interval", noInterval).
			Msg("Creating new alarm state file for service")

		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
		if err != nil {
			log.Error().
				Err(err).
				Str("component", "alarm").
				Str("action", "file_create").
				Str("file_path", filePath).
				Msg("Error creating alarm state file")
			return
		}
		defer file.Close()

		jsonData, err := json.Marshal(&ServiceFile{Date: currentDate, Locked: false})
		if err != nil {
			log.Error().
				Err(err).
				Str("component", "alarm").
				Str("action", "json_marshal").
				Msg("Error marshalling alarm state JSON")
			return
		}

		err = os.WriteFile(filePath, jsonData, 0644)
		if err != nil {
			log.Error().
				Err(err).
				Str("component", "alarm").
				Str("action", "file_write").
				Str("file_path", filePath).
				Msg("Error writing alarm state file")
			return
		}

		if Config.Alarm.Interval == 0 || noInterval {
			log.Debug().
				Str("component", "alarm").
				Str("action", "immediate_alarm").
				Str("service", service).
				Float64("interval", Config.Alarm.Interval).
				Bool("no_interval", noInterval).
				Dur("processing_time", time.Since(startTime)).
				Msg("Sending immediate alarm for service down")
			Alarm(messageFinal, customStream, customTopic, false)
		} else {
			log.Debug().
				Str("component", "alarm").
				Str("action", "defer_alarm").
				Str("service", service).
				Float64("interval_minutes", Config.Alarm.Interval).
				Msg("Service down alarm deferred due to interval setting")
		}
	}
}

type ResponseData struct {
	Result string `json:"result"`
	Msg    string `json:"msg"`
	Code   string `json:"code"`
}

func Alarm(m string, customStream string, customTopic string, onlyFirstWebhook bool) {
	startTime := time.Now()

	if !Config.Alarm.Enabled {
		log.Debug().
			Str("component", "alarm").
			Str("action", "send").
			Bool("enabled", Config.Alarm.Enabled).
			Msg("Alarm system disabled, skipping notification")
		return
	}

	message := strings.Replace(m, "\n", `\n`, -1)
	body := []byte(`{"text":"` + message + `"}`)

	log.Debug().
		Str("component", "alarm").
		Str("action", "send").
		Str("message", message).
		Str("custom_stream", customStream).
		Str("custom_topic", customTopic).
		Bool("only_first_webhook", onlyFirstWebhook).
		Int("webhook_count", len(Config.Alarm.Webhook_urls)).
		Msg("Starting alarm notification")

	successCount := 0
	errorCount := 0

	for i, webhook_url := range Config.Alarm.Webhook_urls {
		webhookStartTime := time.Now()

		if customStream != "" {
			if strings.Contains(webhook_url, "&stream=") {
				re := regexp.MustCompile(`&stream=[^&]*`)
				webhook_url = re.ReplaceAllString(webhook_url, "&stream="+customStream)
			} else {
				webhook_url = webhook_url + "&stream=" + customStream
			}
		}

		if customTopic != "" {
			if strings.Contains(webhook_url, "&topic=") {
				re := regexp.MustCompile(`&topic=[^&]*`)
				webhook_url = re.ReplaceAllString(webhook_url, "&topic="+customTopic)
			} else {
				webhook_url = webhook_url + "&topic=" + customTopic
			}
		}

		r, err := http.NewRequest("POST", webhook_url, bytes.NewBuffer(body))
		r.Header.Set("Content-Type", "application/json")

		if err != nil {
			log.Error().
				Err(err).
				Str("component", "alarm").
				Str("action", "create_request").
				Str("webhook_url", webhook_url).
				Int("webhook_index", i).
				Msg("Error creating HTTP request for alarm")
			errorCount++
			continue
		}

		res, err := http.DefaultClient.Do(r)

		if err != nil {
			log.Error().
				Err(err).
				Str("component", "alarm").
				Str("action", "send_request").
				Str("webhook_url", webhook_url).
				Int("webhook_index", i).
				Dur("request_duration", time.Since(webhookStartTime)).
				Msg("Error sending HTTP request for alarm")
			errorCount++
			continue
		}

		responseBody, err := io.ReadAll(res.Body)

		if err != nil {
			log.Error().
				Err(err).
				Str("component", "alarm").
				Str("action", "read_response").
				Str("webhook_url", webhook_url).
				Int("webhook_index", i).
				Int("status_code", res.StatusCode).
				Msg("Error reading alarm response body")
			res.Body.Close()
			errorCount++
			continue
		}

		var data ResponseData
		err = json.Unmarshal(responseBody, &data)

		if err != nil {
			log.Error().
				Err(err).
				Str("component", "alarm").
				Str("action", "parse_response").
				Str("webhook_url", webhook_url).
				Int("webhook_index", i).
				Str("response_body", string(responseBody)).
				Msg("Error parsing alarm response JSON")
			res.Body.Close()
			errorCount++
			continue
		}

		if data.Result != "success" {
			log.Error().
				Str("component", "alarm").
				Str("action", "webhook_error").
				Str("webhook_url", webhook_url).
				Int("webhook_index", i).
				Str("result", data.Result).
				Str("error_code", data.Code).
				Str("error_message", data.Msg).
				Str("request_json", string(body)).
				Int("status_code", res.StatusCode).
				Dur("request_duration", time.Since(webhookStartTime)).
				Msg("Webhook returned error for alarm")
			errorCount++
		} else {
			log.Debug().
				Str("component", "alarm").
				Str("action", "webhook_success").
				Str("webhook_url", webhook_url).
				Int("webhook_index", i).
				Int("status_code", res.StatusCode).
				Dur("request_duration", time.Since(webhookStartTime)).
				Msg("Alarm sent successfully to webhook")
			successCount++
		}

		res.Body.Close()

		if onlyFirstWebhook {
			log.Debug().
				Str("component", "alarm").
				Str("action", "single_webhook").
				Int("webhook_index", i).
				Msg("Only first webhook requested, stopping after first attempt")
			break
		}
	}

	log.Debug().
		Str("component", "alarm").
		Str("action", "send_complete").
		Str("message", message).
		Int("success_count", successCount).
		Int("error_count", errorCount).
		Int("total_webhooks", len(Config.Alarm.Webhook_urls)).
		Dur("total_duration", time.Since(startTime)).
		Msg("Alarm notification process completed")
}

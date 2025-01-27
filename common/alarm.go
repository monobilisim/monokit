package common

import (
    "io"
    "os"
    "time"
    "bytes"
    "strings"
    "net/http"
    "encoding/json"
    "github.com/spf13/cobra"
)

var TmpDir = "/tmp/mono/"
var ScriptName string

var AlarmCmd = &cobra.Command{
    Use: "alarm",
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
        Alarm(message, "", "", false)
    },
}

func AlarmCheckUp(service string, message string, noInterval bool) {
    // Remove slashes from service and replace them with -
    serviceReplaced := strings.Replace(service, "/", "-", -1)
    file_path := TmpDir + "/" + serviceReplaced + ".log"
    messageFinal := "[" + ScriptName + " - " + Config.Identifier + "] [:check:] " + message
    
    if _, err := os.Stat(file_path); os.IsNotExist(err) {
        return
    }

    // Open file and load the JSON
    file, err := os.OpenFile(file_path, os.O_RDONLY, 0644)
    defer file.Close()

    if err != nil {
        LogError("Error opening file for writing: \n" + err.Error())
    }

    var j ServiceFile

    fileRead, err := io.ReadAll(file)

    if err != nil {
        LogError("Error reading file: \n" + err.Error())
        return
    }

    err = json.Unmarshal(fileRead, &j)

    if err != nil {
        LogError("Error parsing JSON: \n" + err.Error())
        return
    }

    if j.Locked == false && noInterval == false {
        os.Remove(file_path)
        return
    } else {
        os.Remove(file_path)
        Alarm(messageFinal, "", "", false)
    }
}

type ServiceFile struct {
    Date string `json:"date"`
    Locked bool `json:"locked"`
}


func AlarmCheckDown(service string, message string, noInterval bool, customStream string, customTopic string) {
    // Remove slashes from service and replace them with -
    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := TmpDir + "/" + serviceReplaced + ".log"
    currentDate := time.Now().Format("2006-01-02 15:04:05 -0700")

    messageFinal := "[" + ScriptName + " - " + Config.Identifier + "] [:red_circle:] " + message
    
    // Check if the file exists
    if _, err := os.Stat(filePath); err == nil && noInterval == false {
        // Open file and load the JSON
        
        file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
        defer file.Close()

        if err != nil {
            LogError("Error opening file for writing: \n" + err.Error())
        }

        var j ServiceFile

        fileRead, err := io.ReadAll(file)

        if err != nil {
            LogError("Error reading file: \n" + err.Error())
            return
        }

        err = json.Unmarshal(fileRead, &j)

        if err != nil {
            LogError("Error parsing JSON: \n" + err.Error())
            return
        }
    
        // Return if locked == true 
        if j.Locked == true {
            return
        }
       
        oldDate := j.Date
        oldDateParsed, err := time.Parse("2006-01-02 15:04:05 -0700", oldDate)

        if err != nil {
            LogError("Error parsing date: \n" + err.Error())
        }

        finJson := &ServiceFile{
                    Date: currentDate, 
                    Locked: true,
                 }
        
        if Config.Alarm.Interval == 0 {
            if oldDateParsed.Format("2006-01-02") != time.Now().Format("2006-01-02") {
                jsonData, err := json.Marshal(&ServiceFile{Date: currentDate, Locked: false})

                if err != nil {
                    LogError("Error marshalling JSON: \n" + err.Error())
                }

                err = os.WriteFile(filePath, jsonData, 0644)

                Alarm(messageFinal, customStream, customTopic, false)
            }
            return
        }


        if (time.Now().Sub(oldDateParsed).Hours() > 24) {
            jsonData, err := json.Marshal(finJson)
            
            if err != nil {
                LogError("Error marshalling JSON: \n" + err.Error())
            }

            err = os.WriteFile(filePath, jsonData, 0644)

            if err != nil {
                LogError("Error writing to file: \n" + err.Error())
            }
            
            Alarm(messageFinal, customStream, customTopic, false)
        } else {
            if j.Locked == false {
                // currentDate - oldDate in minutes
                timeDiff := time.Now().Sub(oldDateParsed) //.Minutes()

                if timeDiff.Minutes() >= Config.Alarm.Interval { 
                    jsonData, err := json.Marshal(finJson)
                    if err != nil {
                        LogError("Error marshalling JSON: \n" + err.Error())
                    }

                    err = os.WriteFile(filePath, jsonData, 0644)

                    if err != nil {
                        LogError("Error writing to file: \n" + err.Error())
                    }

                    Alarm(messageFinal, customStream, customTopic, false)
                }
            }
        }
    } else {

        file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
        defer file.Close() 

        if err != nil {
            LogError("Error opening file for writing: \n" + err.Error())
            return
        }
        
        jsonData, err := json.Marshal(&ServiceFile{Date: currentDate, Locked: false})
        
        if err != nil {
            LogError("Error marshalling JSON: \n" + err.Error())
        }


        err = os.WriteFile(filePath, jsonData, 0644)

        if err != nil {
            LogError("Error writing to file: \n" + err.Error())
        }


        if Config.Alarm.Interval == 0 || noInterval == true {
            Alarm(messageFinal, customStream, customTopic, false)
        }
    }        
}

type ResponseData struct {
    Result string `json:"result"`
    Msg string `json:"msg"`
    Code string `json:"code"`
}

func Alarm(m string, customStream string, customTopic string, onlyFirstWebhook bool) {
    if Config.Alarm.Enabled == false {
        return
    }

    message := strings.Replace(m, "\n", `\n`, -1)

    body:= []byte(`{"text":"` + message + `"}`)

    for _, webhook_url := range Config.Alarm.Webhook_urls {

		if customStream != "" && customTopic != "" {
			// Remove everything after &
			webhook_url = strings.Split(webhook_url, "&")[0]
			webhook_url = webhook_url + "&stream=" + customStream + "&topic=" + customTopic
		}

        r, err := http.NewRequest("POST", webhook_url, bytes.NewBuffer(body))
        r.Header.Set("Content-Type", "application/json")

        if err != nil {
            LogError("Error creating request for the alarm: \n" + err.Error())
        }

        res, err := http.DefaultClient.Do(r)
        
        if err != nil {
            LogError("Error sending request for the alarm: \n" + err.Error())
        }

        responseBody, err := io.ReadAll(res.Body)
        
        if err != nil {
            LogError("Error reading response for the alarm: \n" + err.Error())
        }

        var data ResponseData

        err = json.Unmarshal(responseBody, &data)

        if err != nil {
            LogError("Error parsing JSON for the alarm: \n" + err.Error())
        }

        if data.Result != "success" {
            LogError("Error sending alarm (" + data.Code + "): \n" + data.Msg)
            LogError("Request JSON: \n" + string(body))
        }

        defer res.Body.Close()

		if onlyFirstWebhook == true {
			break
		}
    }
}

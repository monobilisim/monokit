package common

import (
    "bytes"
    "net/http"
    "time"
    "encoding/json"
    "io"
    "os"
    "strings"
)

var TmpDir = "/tmp/mono/"
var ScriptName string

func AlarmCheckUp(service string, message string) {
    // Remove slashes from service and replace them with -
    serviceReplaced := strings.Replace(service, "/", "-", -1)
    file_path := TmpDir + "/" + serviceReplaced + ".log"
    messageFinal := "[" + ScriptName + " - " + Config.Identifier + "] [:red_circle:] " + message
    
    // Check if the file exists, send alarm and remove file if it does
    if _, err := os.Stat(file_path); err == nil {
        os.Remove(file_path)
        Alarm(messageFinal)
    }
}

type serviceFile struct {
    Date string `json:"date"`
    Locked bool `json:"locked"`
}


func AlarmCheckDown(service string, message string) {
    // Remove slashes from service and replace them with -
    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := TmpDir + "/" + serviceReplaced + ".log"
    currentDate := time.Now().Format("2006-01-02 15:04:05 -0700")

    messageFinal := "[" + ScriptName + " - " + Config.Identifier + "] [:red_circle:] " + message
    
    // Check if the file exists
    if _, err := os.Stat(filePath); err == nil {
        // Open file and load the JSON
        
        file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
        defer file.Close()

        if err != nil {
            LogError("Error opening file for writing: \n" + err.Error())
        }

        var j serviceFile

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

        finJson := &serviceFile{
                    Date: currentDate, 
                    Locked: true,
                 }
        
        if Config.Alarm.Interval == 0 {
            if oldDateParsed.Format("2006-01-02") != time.Now().Format("2006-01-02") {
                jsonData, err := json.Marshal(&serviceFile{Date: currentDate, Locked: false})

                if err != nil {
                    LogError("Error marshalling JSON: \n" + err.Error())
                }

                err = os.WriteFile(filePath, jsonData, 0644)

                Alarm(messageFinal)
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
            
            Alarm(messageFinal)
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

                    Alarm(messageFinal)
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
        
        jsonData, err := json.Marshal(&serviceFile{Date: currentDate, Locked: false})
        
        if err != nil {
            LogError("Error marshalling JSON: \n" + err.Error())
        }


        err = os.WriteFile(filePath, jsonData, 0644)

        if err != nil {
            LogError("Error writing to file: \n" + err.Error())
        }


        if Config.Alarm.Interval == 0 {
            Alarm(messageFinal)
        }
    }        
}

func Alarm(message string) {
    if Config.Alarm.Enabled == false {
        return
    }

    body:= []byte(`{"text":"` + message + `"}`)

    for _, webhook_url := range Config.Alarm.Webhook_urls {
        r, err := http.NewRequest("POST", webhook_url, bytes.NewBuffer(body))
        r.Header.Set("Content-Type", "application/json")

        if err != nil {
            LogError("Error creating request for the alarm: \n" + err.Error())
        }

        res, err := http.DefaultClient.Do(r)
        
        if err != nil {
            LogError("Error sending request for the alarm: \n" + err.Error())
        }

        defer res.Body.Close()
    }
}

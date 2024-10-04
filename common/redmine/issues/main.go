package common

import (
    "strconv"
    "bytes"
    "io"
    "net/http"
    "time"
    "os"
    "encoding/json"
    "strings"
    "github.com/monobilisim/monokit/common"
)

type Issue struct {
        Id             int       `json:"id,omitempty"`
        Notes          string    `json:"notes,omitempty"`
        ProjectId      string    `json:"project_id,omitempty"`
        TrackerId      int       `json:"tracker_id,omitempty"`
        Description    string    `json:"description,omitempty"`
        Subject        string    `json:"subject,omitempty"`
        PriorityId     int       `json:"priority_id,omitempty"`
        StatusId       int       `json:"status_id,omitempty"`
        AssignedToId   string       `json:"assigned_to_id,omitempty"`
} 

type RedmineIssue struct {
    Issue Issue `json:"issue"`
}

func redmineCheckIssueLog(service string) bool {
    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"
    
    // If file exists, return
    if _, err := os.Stat(filePath); err == nil {
        // Check if file is empty, if so delete the file and return
        if common.IsEmptyOrWhitespace(filePath) {
            err := os.Remove(filePath)
            if err != nil {
                common.LogError("os.Remove error: " + err.Error())
            }
            return false
        }

        // Check if file is 0, if so delete the file and return
        read, err := os.ReadFile(filePath)

        if err != nil {
            common.LogError("os.ReadFile error: " + err.Error())
        }

        if string(read) == "0" {
            err := os.Remove(filePath)
            if err != nil {
                common.LogError("os.Remove error: " + err.Error())
            }
            return false
        }
        
        return true
    }

    return false
}

func redmineWrapper(service string, subject string, message string) {
    
    if redmineCheckIssueLog(service) == false {
        Create(service, subject, message)
    } else {
        Update(service, message, true)
    }
}


func CheckUp(service string, message string) {
    // Remove slashes from service and replace them with -
    serviceReplaced := strings.Replace(service, "/", "-", -1)
    file_path := common.TmpDir + "/" + serviceReplaced + "-redmine-stat.log"

    // Check if the file exists, close issue and remove file if it does
    if _, err := os.Stat(file_path); err == nil {
        os.Remove(file_path)
        Close(service, message)
    }
}

func CheckDown(service string, subject string, message string) {
    // Remove slashes from service and replace them with -
    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := common.TmpDir + "/" + serviceReplaced + "-redmine-stat.log"
    currentDate := time.Now().Format("2006-01-02 15:04:05 -0700")

    // Check if the file exists
    if _, err := os.Stat(filePath); err == nil {
        // Open file and load the JSON

        file, err := os.OpenFile(filePath, os.O_RDONLY, 0644)
        defer file.Close()

        if err != nil {
            common.LogError("Error opening file for writing: \n" + err.Error())
        }

        var j common.ServiceFile

        fileRead, err := io.ReadAll(file)

        if err != nil {
            common.LogError("Error reading file: \n" + err.Error())
            return
        }

        err = json.Unmarshal(fileRead, &j)

        if err != nil {
            common.LogError("Error parsing JSON: \n" + err.Error())
            return
        }

        // Return if locked == true
        if j.Locked == true {
            return
        }

        oldDate := j.Date
        oldDateParsed, err := time.Parse("2006-01-02 15:04:05 -0700", oldDate)

        if err != nil {
            common.LogError("Error parsing date: \n" + err.Error())
        }

        finJson := &common.ServiceFile{
                    Date: currentDate,
                    Locked: true,
                 }

        if common.Config.Redmine.Interval == 0 {
            if oldDateParsed.Format("2006-01-02") != time.Now().Format("2006-01-02") {
                jsonData, err := json.Marshal(&common.ServiceFile{Date: currentDate, Locked: false})

                if err != nil {
                    common.LogError("Error marshalling JSON: \n" + err.Error())
                }

                err = os.WriteFile(filePath, jsonData, 0644)
                
                redmineWrapper(service, subject, message)
            }
            return
        }


        if (time.Now().Sub(oldDateParsed).Hours() > 24) {
            jsonData, err := json.Marshal(finJson)

            if err != nil {
                common.LogError("Error marshalling JSON: \n" + err.Error())
            }

            err = os.WriteFile(filePath, jsonData, 0644)

            if err != nil {
                common.LogError("Error writing to file: \n" + err.Error())
            }

            redmineWrapper(service, subject, message)
        } else {
            if j.Locked == false {
                // currentDate - oldDate in minutes
                timeDiff := time.Now().Sub(oldDateParsed) //.Minutes()

                if timeDiff.Minutes() >= common.Config.Redmine.Interval {
                    jsonData, err := json.Marshal(finJson)
                    if err != nil {
                        common.LogError("Error marshalling JSON: \n" + err.Error())
                    }

                    err = os.WriteFile(filePath, jsonData, 0644)

                    if err != nil {
                        common.LogError("Error writing to file: \n" + err.Error())
                    }
                   
                    redmineWrapper(service, subject, message)
                }
            }
        }
    } else {

        file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
        defer file.Close()

        if err != nil {
            common.LogError("Error opening file for writing: \n" + err.Error())
            return
        }

        jsonData, err := json.Marshal(&common.ServiceFile{Date: currentDate, Locked: false})

        if err != nil {
            common.LogError("Error marshalling JSON: \n" + err.Error())
        }


        err = os.WriteFile(filePath, jsonData, 0644)

        if err != nil {
            common.LogError("Error writing to file: \n" + err.Error())
        }


        if common.Config.Redmine.Interval == 0 {
            redmineWrapper(service, subject, message)
        }
    }
}

func Create(service string, subject string, message string) {
    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"
   
    if common.Config.Redmine.Enabled == false {
        return
    }
    
    if redmineCheckIssueLog(service) == true {
        return
    }

    var priorityId int
    var projectId string

    if common.Config.Redmine.Priority_id == 0 {
        priorityId = 5
    } else {
        priorityId = common.Config.Redmine.Priority_id
    }

    if common.Config.Redmine.Project_id == "" {
        projectId = strings.Split(common.Config.Identifier, "-")[0]
    } else {
        projectId = common.Config.Redmine.Project_id
    }

    body := RedmineIssue{Issue: Issue{ProjectId: projectId, TrackerId: 7, Description: message, Subject: subject, PriorityId: priorityId }}

    jsonBody, err := json.Marshal(body)

    if err != nil {
        common.LogError("json.Marshal error: " + err.Error())
    }

    req, err := http.NewRequest("POST", common.Config.Redmine.Url + "/issues.json", bytes.NewBuffer(jsonBody))

    if err != nil {
        common.LogError("http.NewRequest error: " + err.Error())
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

    client := &http.Client{
        Timeout: time.Second * 10,
    }

    resp, err := client.Do(req)

    if err != nil {
        common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + common.Config.Redmine.Url + "/issues.json" + "\n" + "Redmine JSON: " + string(jsonBody))
        return
    }

    defer resp.Body.Close()

    // read response
    var data RedmineIssue

    err = json.NewDecoder(resp.Body).Decode(&data)

    if err != nil {
        common.LogError("json.NewDecoder error: " + err.Error())
    }

    // get issue id, convert to string
    issueId := []byte(strconv.Itoa(data.Issue.Id))

    // write issue id to file
    err = os.WriteFile(filePath, issueId, 0644)

    if err != nil {
        common.LogError("os.WriteFile error while trying to read '" + filePath + "'" + err.Error())
    }
}

func ExistsNote(service string, message string) bool {
    // Check if a note in an issue already exists
    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

    // check if filePath exists, if not return
    if _, err := os.Stat(filePath); os.IsNotExist(err) {
        return false
    }

    // Check if file is empty, if so delete the file and return
    if common.IsEmptyOrWhitespace(filePath) {
        err := os.Remove(filePath)
        if err != nil {
            common.LogError("os.Remove error: " + err.Error())
        }
        return false
    }

    // read file
    file, err := os.ReadFile(filePath)

    if err != nil {
        common.LogError("os.ReadFile error: " + err.Error())
        return false
    }

    if string(file) == "0" {
        err := os.Remove(filePath)
        if err != nil {
            common.LogError("os.Remove error: " + err.Error())
        }
    }

    redmineUrlFinal := common.Config.Redmine.Url + "/issues/" + string(file) + ".json?include=journals"

    // Send a GET request to the Redmine API to get all issues
    req, err := http.NewRequest("GET", redmineUrlFinal, nil)

    if err != nil {
        common.LogError("http.NewRequest error: " + err.Error())
        return false
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

    client := &http.Client{
        Timeout: time.Second * 10,
    }

    resp, err := client.Do(req)

    if err != nil {
        common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + redmineUrlFinal)
        return false
    }

    defer resp.Body.Close()

    // read response and get notes
    var data map[string]interface{}

    err = json.NewDecoder(resp.Body).Decode(&data)

    if err != nil {
        common.LogError("json.NewDecoder error: " + err.Error())
        return false
    }

    // If not 200, log error
    if resp.StatusCode != 200 {
        // Unmarshal the response body
        common.LogError("Redmine API returned status code " + strconv.Itoa(resp.StatusCode) + " instead of 200\n" + "Redmine URL: " + redmineUrlFinal)
        return false
    }

    // Check if the note already exists
    for _, journal := range data["issue"].(map[string]interface{})["journals"].([]interface{}) {
        if journal.(map[string]interface{})["notes"].(string) == message {
            return true
        }
    }

    return false
}





func Update(service string, message string, checkNote bool) {
    
    if common.Config.Redmine.Enabled == false {
        return
    }

    if checkNote {
        if ExistsNote(service, message) {
            return
        }
    }

    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"
    
    if redmineCheckIssueLog(service) == false {
        return
    }

    // read file
    file, err := os.ReadFile(filePath)

    if err != nil {
        common.LogError("os.ReadFile error: " + err.Error())
    }

    // get issue id
    issueId, err := strconv.Atoi(string(file))

    if err != nil {
        common.LogError("strconv.Atoi error: " + err.Error())
    }

    if issueId == 0 {
        // Remove file
        err := os.Remove(filePath)
        if err != nil {
            common.LogError("os.Remove error: " + err.Error())
        }
        return
    }

    // update issue
    body := RedmineIssue{Issue: Issue{Id: issueId, Notes: message}}

    jsonBody, err := json.Marshal(body)

    if err != nil {
        common.LogError("json.Marshal error: " + err.Error())
    }

    req, err := http.NewRequest("PUT", common.Config.Redmine.Url + "/issues/" + string(file) + ".json", bytes.NewBuffer(jsonBody))

    if err != nil {
        common.LogError("http.NewRequest error: " + err.Error())
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

    client := &http.Client{
        Timeout: time.Second * 10,
    }

    resp, err := client.Do(req)

    if err != nil {
        common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + common.Config.Redmine.Url + "/issues/" + string(file) + ".json" + "\n" + "Redmine JSON: " + string(jsonBody))
        return
    }

    defer resp.Body.Close()
}


func getAssignedToId(id string) string {

    // Make request to Redmine API to get the assigned_to_id
    redmineUrlFinal := common.Config.Redmine.Url + "/issues/" + id + ".json"

    req, err := http.NewRequest("GET", redmineUrlFinal, nil)
   
    if err != nil {
        common.LogError("http.NewRequest error: " + err.Error())
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

    client := &http.Client{
        Timeout: time.Second * 10,
    }

    resp, err := client.Do(req)

    if err != nil {
        common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + redmineUrlFinal)
        return ""
    }

    defer resp.Body.Close()

    // read response and get assigned_to_id
    var data map[string]interface{}

    err = json.NewDecoder(resp.Body).Decode(&data)

    if err != nil {
        common.LogError("json.NewDecoder error: " + err.Error())
    }

    // If not 200, log error

    if resp.StatusCode != 200 {
        // Unmarshal the response body
        common.LogError("Redmine API returned status code " + strconv.Itoa(resp.StatusCode) + " instead of 200\n" + "Redmine URL: " + redmineUrlFinal)
        return ""
    }

    info, err := strconv.Itoa(int(data["issue"].(map[string]interface{})["assigned_to"].(map[string]interface{})["id"].(float64)))

    if err != nil {
        return "" // just means that the issue is assigned to nobody
    }

    return info
}


func Close(service string, message string) {
    if common.Config.Redmine.Enabled == false {
        return
    }

    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

    // check if filePath exists, if not return
    if _, err := os.Stat(filePath); os.IsNotExist(err) {
        return
    }
    
    if redmineCheckIssueLog(service) == false {
        return
    }

    // read file
    file, err := os.ReadFile(filePath)
    if err != nil {
        common.LogError("os.ReadFile error while trying to read '" + filePath + "'" + err.Error())
    }

    issueId, err := strconv.Atoi(string(file))

    if err != nil {
        common.LogError("strconv.Atoi error: " + err.Error())
    }

    if issueId == 0 {
        // Remove file
        err := os.Remove(filePath)
        if err != nil {
            common.LogError("os.Remove error: " + err.Error())
        }
        return
    }

    assignedToId := getAssignedToId(string(file))

    if assignedToId == "" {
        assignedToId = "me"
    }

    // update issue
    body := RedmineIssue{Issue: Issue{Id: issueId, Notes: message, StatusId: 5, AssignedToId: assignedToId}}
    jsonBody, err := json.Marshal(body)

    if err != nil {
        common.LogError("json.Marshal error: " + err.Error())
    }


    req, err := http.NewRequest("PUT", common.Config.Redmine.Url + "/issues/" + string(file) + ".json", bytes.NewBuffer(jsonBody))

    if err != nil {
        common.LogError("http.NewRequest error: " + err.Error())
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

    client := &http.Client{
        Timeout: time.Second * 10,
    }

    resp, err := client.Do(req)

    if err != nil {
        common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + common.Config.Redmine.Url + "/issues/" + string(file) + ".json" + "\n" + "Redmine JSON: " + string(jsonBody))
        return
    }

    defer resp.Body.Close()

    // remove file
    err = os.Remove(filePath)

    if err != nil {
        common.LogError("os.Remove error: " + err.Error())
    }
}

func Show(service string) string {
    if common.Config.Redmine.Enabled == false {
        return ""
    }

    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := common.TmpDir + "/" + serviceReplaced + "-redmine.log"

    if redmineCheckIssueLog(service) == false {
        return ""
    }

    // read file
    file, err := os.ReadFile(filePath)
    if err != nil {
        common.LogError("os.ReadFile error: " + err.Error())
    }

    // get issue ID
    return string(file)
}

func Exists(subject string, date string, search bool) string {
    var projectId string

    if common.Config.Redmine.Project_id == "" {
        projectId = strings.Split(common.Config.Identifier, "-")[0]
    } else {
        projectId = common.Config.Redmine.Project_id
    }

    if common.Config.Redmine.Enabled == false {
        return ""
    }

    subject = strings.Replace(subject, " ", "%20", -1)
   
    redmineUrlFinal := common.Config.Redmine.Url + "/issues.json?project_id=" + projectId

    if search {
        redmineUrlFinal += "&subject=~" + subject
    } else {
        redmineUrlFinal += "&subject=" + subject
    }

    if date != "" {
        redmineUrlFinal += "&created_on=" + date
    }

    // Send a GET request to the Redmine API to get all issues
    req, err := http.NewRequest("GET", redmineUrlFinal, nil)


    if err != nil {
        common.LogError("http.NewRequest error: " + err.Error())
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

    client := &http.Client{
        Timeout: time.Second * 10,
    }

    resp, err := client.Do(req)

    if err != nil {
        common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + redmineUrlFinal)
        return ""
    }

    defer resp.Body.Close()

    // read response and get issue ID
    var data map[string]interface{}

    err = json.NewDecoder(resp.Body).Decode(&data)

    if err != nil {
        common.LogError("json.NewDecoder error: " + err.Error())
    }

    // If not 200, log error
    if resp.StatusCode != 200 {
        // Unmarshal the response body
        common.LogError("Redmine API returned status code " + strconv.Itoa(resp.StatusCode) + " instead of 200\n" + "Redmine URL: " + redmineUrlFinal)
        return ""
    }

    if data["total_count"] == nil || data["total_count"].(float64) == 0 {
        return ""
    } else {
        if data["issues"].([]interface{})[0].(map[string]interface{})["status"].(map[string]interface{})["id"].(float64) == 5 {
            return ""
        } else {
            return strconv.Itoa(int(data["issues"].([]interface{})[0].(map[string]interface{})["id"].(float64)))
    }
}
}


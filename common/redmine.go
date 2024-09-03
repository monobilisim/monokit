package common

import (
    "github.com/spf13/cobra"
    "strconv"
    "bytes"
    "net/http"
    "time"
    "os"
    "encoding/json"
    "strings"
    "fmt"
)

var RedmineCmd = &cobra.Command{
    Use:   "redmine",
    Short: "Redmine-related utilities",
}

var RedmineIssueCmd = &cobra.Command{
    Use:   "issue",
    Short: "Issue-related utilities",
}

var RedmineCreateCmd = &cobra.Command{
    Use:   "create",
    Short: "Create a new issue in Redmine",
    Run: func(cmd *cobra.Command, args []string) {
        Init()
        service, _ := cmd.Flags().GetString("service")
        subject, _ := cmd.Flags().GetString("subject")
        message, _ := cmd.Flags().GetString("message")
        RedmineCreate(service, subject, message)
    },
}

var RedmineUpdateCmd = &cobra.Command{
    Use:   "update",
    Short: "Update an existing issue in Redmine",
    Run: func(cmd *cobra.Command, args []string) {
        Init()
        service, _ := cmd.Flags().GetString("service")
        message, _ := cmd.Flags().GetString("message")
        RedmineUpdate(service, message)
    },
}

var RedmineCloseCmd = &cobra.Command{
    Use:   "close",
    Short: "Close an existing issue in Redmine",
    Run: func(cmd *cobra.Command, args []string) {
        Init()
        service, _ := cmd.Flags().GetString("service")
        message, _ := cmd.Flags().GetString("message")
        RedmineClose(service, message)
    },
}

var RedmineShowCmd = &cobra.Command{
    Use:   "show",
    Short: "Get the issue ID of the issue if it is opened",
    Run: func(cmd *cobra.Command, args []string) {
        Init()
        service, _ := cmd.Flags().GetString("service")
        fmt.Println(RedmineShow(service))
    },
}

type Issue struct {
        Id             int       `json:"id,omitempty"`
        Notes          string    `json:"notes,omitempty"`
        ProjectId      string    `json:"project_id,omitempty"`
        TrackerId      int       `json:"tracker_id,omitempty"`
        Description    string    `json:"description,omitempty"`
        Subject        string    `json:"subject,omitempty"`
        PriorityId     int       `json:"priority_id,omitempty"`
        StatusId       int       `json:"status_id,omitempty"`
} 

type RedmineIssue struct {
    Issue Issue `json:"issue"`
}


func RedmineCreate(service string, subject string, message string) {
    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := TmpDir + "/" + serviceReplaced + "-redmine.log"
   
    if Config.Redmine.Enabled == false {
        return
    }
        
    // If file exists, return
    if _, err := os.Stat(filePath); err == nil {
        return
    }

    var priorityId int
    var projectId string

    if Config.Redmine.Priority_id == 0 {
        priorityId = 5
    } else {
        priorityId = Config.Redmine.Priority_id
    }
    
    if Config.Redmine.Project_id == "" {
        projectId = strings.Split(Config.Identifier, "-")[0]
    } else {
        projectId = Config.Redmine.Project_id
    }

    body := RedmineIssue{Issue: Issue{ProjectId: projectId, TrackerId: 7, Description: message, Subject: subject, PriorityId: priorityId }}

    jsonBody, err := json.Marshal(body)

    if err != nil {
        LogError("json.Marshal error: " + err.Error())
    }

    req, err := http.NewRequest("POST", Config.Redmine.Url + "/issues.json", bytes.NewBuffer(jsonBody))

    if err != nil {
        LogError("http.NewRequest error: " + err.Error())
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Redmine-API-Key", Config.Redmine.Api_key)

    client := &http.Client{
        Timeout: time.Second * 10,
    }

    resp, err := client.Do(req)

    if err != nil {
        LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + Config.Redmine.Url + "/issues.json" + "\n" + "Redmine JSON: " + string(jsonBody))
        return
    }

    defer resp.Body.Close()

    // read response
    var data RedmineIssue

    err = json.NewDecoder(resp.Body).Decode(&data)

    if err != nil {
        LogError("json.NewDecoder error: " + err.Error())
    }

    // get issue id, convert to string
    issueId := []byte(strconv.Itoa(data.Issue.Id))

    // write issue id to file
    err = os.WriteFile(filePath, issueId, 0644)

    if err != nil {
        LogError("os.WriteFile error while trying to read '" + filePath + "'" + err.Error())
    }
}

func RedmineUpdate(service string, message string) {
    
    if Config.Redmine.Enabled == false {
        return
    }

    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := TmpDir + "/" + serviceReplaced + "-redmine.log"
    
    // check if filePath exists
    if _, err := os.Stat(filePath); os.IsNotExist(err) {
        return
    }

    // read file
    file, err := os.ReadFile(filePath)

    if err != nil {
        LogError("os.ReadFile error: " + err.Error())
    }

    // get issue id
    issueId, err := strconv.Atoi(string(file))

    if err != nil {
        LogError("strconv.Atoi error: " + err.Error())
    }

    // update issue
    body := RedmineIssue{Issue: Issue{Id: issueId, Notes: message}}

    jsonBody, err := json.Marshal(body)

    if err != nil {
        LogError("json.Marshal error: " + err.Error())
    }

    req, err := http.NewRequest("PUT", Config.Redmine.Url + "/issues/" + string(file) + ".json", bytes.NewBuffer(jsonBody))

    if err != nil {
        LogError("http.NewRequest error: " + err.Error())
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Redmine-API-Key", Config.Redmine.Api_key)

    client := &http.Client{
        Timeout: time.Second * 10,
    }

    resp, err := client.Do(req)

    if err != nil {
        LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + Config.Redmine.Url + "/issues/" + string(file) + ".json" + "\n" + "Redmine JSON: " + string(jsonBody))
        return
    }

    defer resp.Body.Close()
}

func RedmineClose(service string, message string) {
    if Config.Redmine.Enabled == false {
        return
    }

    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := TmpDir + "/" + serviceReplaced + "-redmine.log"

    // check if filePath exists, if not return
    if _, err := os.Stat(filePath); os.IsNotExist(err) {
        return
    }

    // read file
    file, err := os.ReadFile(filePath)
    if err != nil {
        LogError("os.ReadFile error while trying to read '" + filePath + "'" + err.Error())
    }

    issueId, err := strconv.Atoi(string(file))

    if err != nil {
        LogError("strconv.Atoi error: " + err.Error())
    }

    // update issue
    body := RedmineIssue{Issue: Issue{Id: issueId, Notes: message, StatusId: 5}}
    jsonBody, err := json.Marshal(body)

    if err != nil {
        LogError("json.Marshal error: " + err.Error())
    }


    req, err := http.NewRequest("PUT", Config.Redmine.Url + "/issues/" + string(file) + ".json", bytes.NewBuffer(jsonBody))

    if err != nil {
        LogError("http.NewRequest error: " + err.Error())
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Redmine-API-Key", Config.Redmine.Api_key)

    client := &http.Client{
        Timeout: time.Second * 10,
    }

    resp, err := client.Do(req)

    if err != nil {
        LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + Config.Redmine.Url + "/issues/" + string(file) + ".json" + "\n" + "Redmine JSON: " + string(jsonBody))
        return
    }

    defer resp.Body.Close()

    // remove file
    err = os.Remove(filePath)

    if err != nil {
        LogError("os.Remove error: " + err.Error())
    }
}

func RedmineShow(service string) string {
    if Config.Redmine.Enabled == false {
        return ""
    }

    serviceReplaced := strings.Replace(service, "/", "-", -1)
    filePath := TmpDir + "/" + serviceReplaced + "-redmine.log"

    // check if filePath exists, if not return
    if _, err := os.Stat(filePath); os.IsNotExist(err) {
        return ""
    }

    // read file
    file, err := os.ReadFile(filePath)
    if err != nil {
        LogError("os.ReadFile error: " + err.Error())
    }

    // get issue ID
    return string(file)
}

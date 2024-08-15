package common

import (
    "bytes"
    "net/http"
    "time"
    "os"
    "encoding/json"
    "strings"
)

type Issue struct {
        Id          int       `json:"id"`
        Notes       string    `json:"notes"`
        ProjectId   string    `json:"project_id"`
        TrackerId   int       `json:"tracker_id"`
        Description string    `json:"description"`
        Subject     string    `json:"subject"`
        PriorityId  int       `json:"priority_id"`
        StatusId    string    `json:"status_id"`
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

    body := RedmineIssue{Issue: Issue{ProjectId: projectId, TrackerId: 7, Description: message, Subject: subject, PriorityId: priorityId, StatusId: "open"}}

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
        LogError("client.Do error: " + err.Error())
    }

    defer resp.Body.Close()

    jsonRespBody, err := json.Marshal(resp.Body)

    if err != nil {
        LogError("json.Marshal error: " + err.Error())
    }

    os.WriteFile(filePath, jsonRespBody, 0666)
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

    // parse json
    var data RedmineIssue
    err = json.Unmarshal(file, &data)
    if err != nil {
        LogError("json.Unmarshal error: " + err.Error())
    }

    // get issue id
    issueId := data.Issue.Id

    // update issue
    body := RedmineIssue{Issue: Issue{Id: issueId, Notes: message}}

    jsonBody, err := json.Marshal(body)

    if err != nil {
        LogError("json.Marshal error: " + err.Error())
    }

    req, err := http.NewRequest("PUT", Config.Redmine.Url + "/issues/" + string(issueId) + ".json", bytes.NewBuffer(jsonBody))

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
        LogError("client.Do error: " + err.Error())
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
        LogError("os.ReadFile error: " + err.Error())
    }

    // parse json
    var data RedmineIssue
    err = json.Unmarshal(file, &data)
    if err != nil {
        LogError("json.Unmarshal error: " + err.Error())
    }

    // get issue id
    issueId := data.Issue.Id

    // update issue
    body := RedmineIssue{Issue: Issue{Id: issueId, Notes: message, StatusId: "closed"}}
    jsonBody, err := json.Marshal(body)

    if err != nil {
        LogError("json.Marshal error: " + err.Error())
    }

    req, err := http.NewRequest("PUT", Config.Redmine.Url + "/issues/" + string(issueId) + ".json", bytes.NewBuffer(jsonBody))

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
        LogError("client.Do error: " + err.Error())
    }

    defer resp.Body.Close()

    // remove file
    err = os.Remove(filePath)

    if err != nil {
        LogError("os.Remove error: " + err.Error())
    }
}

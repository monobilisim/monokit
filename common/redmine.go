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

    req, err := http.NewRequest("POST", Config.Redmine.Url, bytes.NewBuffer(jsonBody))

    if err != nil {
        LogError("http.NewRequest error: " + err.Error())
    }

    req.Header.Set("Content-Type", "application/json")

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
    // TODO
}

func RedmineDisable(service string) {
    // TODO
}

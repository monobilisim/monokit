package common

import (
    "bytes"
    "net/http"
    "time"
    "encoding/json"
    "strings"
    "github.com/monobilisim/monokit/common"
    "io/ioutil"
    "fmt"
)

type News struct {
    Title              string    `json:"title,omitempty"`
    Description        string    `json:"description,omitempty"`
}

type RedmineNews struct {
    News News `json:"news"`
}

func Create(title string, description string) string {
    if common.Config.Redmine.Enabled == false {
        return ""
    }
   
    var projectId string

    if common.Config.Redmine.Project_id == "" {
        projectId = strings.Split(common.Config.Identifier, "-")[0]
    } else {
        projectId = common.Config.Redmine.Project_id
    }

    body := RedmineNews{News: News{Title: title, Description: description}} 

    jsonBody, err := json.Marshal(body)

    if err != nil {
        common.LogError("json.Marshal error: " + err.Error())
    }

    req, err := http.NewRequest("POST", common.Config.Redmine.Url + "/projects/" + projectId + "/news.json", bytes.NewBuffer(jsonBody))

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
        return ""
    }

    defer resp.Body.Close()

    
    newsId := Exists(title, description)

    if newsId == "" {
        common.LogError("News couldn't be created, id returns empty")
        return ""
    } else {
        return newsId
    }

}

func Delete(id string) {
    if common.Config.Redmine.Enabled == false {
        return
    }

    req, err := http.NewRequest("DELETE", common.Config.Redmine.Url + "/news/" + id + ".json", nil)

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
        common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + common.Config.Redmine.Url + "/issues.json")
        return
    }

    defer resp.Body.Close()
}

func Exists(title string, description string) string {
    // Check if the news already exist with the same title and description, return id if exists

    if common.Config.Redmine.Enabled == false {
        return ""
    }

    var projectId string

    if common.Config.Redmine.Project_id == "" {
        projectId = strings.Split(common.Config.Identifier, "-")[0]
    } else {
        projectId = common.Config.Redmine.Project_id
    }

    req, err := http.NewRequest("GET", common.Config.Redmine.Url + "/projects/" + projectId + "/news.json", nil)

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
        common.LogError("client.Do error: " + err.Error() + "\n" + "Redmine URL: " + common.Config.Redmine.Url + "/issues.json")
        return ""
    }

    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)

    if err != nil {
        common.LogError("ioutil.ReadAll error: " + err.Error())
        return ""
    }

    var newsList map[string]interface{}

    err = json.Unmarshal(body, &newsList)

    if err != nil {
        common.LogError("json.Unmarshal error: " + err.Error())
        return ""
    }

    for _, news := range newsList["news"].([]interface{}) {
        if news.(map[string]interface{})["title"] == title && news.(map[string]interface{})["description"] == description {
            return fmt.Sprintf("%v", news.(map[string]interface{})["id"])
        }
    }

    return ""
}

package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

type News struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type RedmineNews struct {
	News News `json:"news"`
}

func Create(title string, description string, noDuplicate bool) string {
	if !common.Config.Redmine.Enabled {
		return ""
	}

	if noDuplicate {
		duplicateId := Exists(title, description)
		if duplicateId != "" {
			return duplicateId
		}
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
		log.Error().Err(err).Str("component", "redmine").Str("operation", "create_news").Str("title", title).Msg("Failed to marshal news creation request")
	}

	req, err := http.NewRequest("POST", common.Config.Redmine.Url+"/projects/"+projectId+"/news.json", bytes.NewBuffer(jsonBody))

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "create_news").Str("title", title).Str("project_id", projectId).Msg("Failed to create news HTTP request")
	}

	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "create_news").Str("title", title).Str("url", common.Config.Redmine.Url+"/projects/"+projectId+"/news.json").Str("request_body", string(jsonBody)).Msg("Failed to send news creation request")
		return ""
	}

	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	newsId := Exists(title, description)

	if newsId == "" {
		log.Error().Str("component", "redmine").Str("operation", "create_news").Str("title", title).Str("url", common.Config.Redmine.Url+"/projects/"+projectId+"/news.json").Str("request_body", string(jsonBody)).Str("response_body", string(respBody)).Msg("Failed to create news - ID not found after creation")
		return ""
	} else {
		return newsId
	}

}

func Delete(id string) {
	if !common.Config.Redmine.Enabled {
		return
	}

	req, err := http.NewRequest("DELETE", common.Config.Redmine.Url+"/news/"+id+".json", nil)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "delete_news").Str("news_id", id).Msg("Failed to create news deletion HTTP request")
	}

	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "delete_news").Str("news_id", id).Str("url", common.Config.Redmine.Url+"/news/"+id+".json").Msg("Failed to send news deletion request")
		return
	}

	defer resp.Body.Close()
}

func Exists(title string, description string) string {
	// Check if the news already exist with the same title and description, return id if exists

	if !common.Config.Redmine.Enabled {
		return ""
	}

	var projectId string

	if common.Config.Redmine.Project_id == "" {
		projectId = strings.Split(common.Config.Identifier, "-")[0]
	} else {
		projectId = common.Config.Redmine.Project_id
	}

	req, err := http.NewRequest("GET", common.Config.Redmine.Url+"/projects/"+projectId+"/news.json", nil)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_news").Str("title", title).Str("project_id", projectId).Msg("Failed to create news search HTTP request")
	}

	common.AddUserAgent(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Redmine-API-Key", common.Config.Redmine.Api_key)

	client := &http.Client{
		Timeout: time.Second * 10,
	}

	resp, err := client.Do(req)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_news").Str("title", title).Str("url", common.Config.Redmine.Url+"/projects/"+projectId+"/news.json").Msg("Failed to send news search request")
		return ""
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_news").Str("title", title).Msg("Failed to read news search response body")
		return ""
	}

	var newsList map[string]interface{}

	err = json.Unmarshal(body, &newsList)

	if err != nil {
		log.Error().Err(err).Str("component", "redmine").Str("operation", "exists_news").Str("title", title).Msg("Failed to unmarshal news search response")
		return ""
	}

	for _, news := range newsList["news"].([]interface{}) {
		if news.(map[string]interface{})["title"] == title && news.(map[string]interface{})["description"] == description {
			return fmt.Sprintf("%v", news.(map[string]interface{})["id"])
		}
	}

	return ""
}

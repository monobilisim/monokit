package uninstall

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

type Project struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Identifier  string    `json:"identifier"`
	Description string    `json:"description"`
	Status      int       `json:"status"`
	CreatedOn   time.Time `json:"created_on"`
}

type RedmineResponse struct {
	Projects   []Project `json:"projects"`
	TotalCount int       `json:"total_count"`
	Offset     int       `json:"offset"`
	Limit      int       `json:"limit"`
}

var ProjectIdentifier string

func Uninstall() {
	common.Init()

	checkDir := "/tmp/mono"
	checkFile := checkDir + "/uninstall_check"

	if err := os.MkdirAll(checkDir, 0755); err != nil {
		log.Error().Err(err).Msg("Failed to create check directory")
	}

	info, err := os.Stat(checkFile)
	if err == nil {
		if time.Since(info.ModTime()) < 24*time.Hour {
			return
		}
	}

	if err := os.WriteFile(checkFile, []byte(time.Now().String()), 0644); err != nil {
		log.Error().Err(err).Msg("Failed to update check file timestamp")
	}

	if isProjectInactive() {
		notify("Uninstalling monokit for host " + common.Config.Identifier)
		removeCron()
		removeFromSSHD()
		removeShutdownNotifier()
		deleteMonokit()
		notify("monokit uninstallation for host " + common.Config.Identifier + " completed.")
		os.Exit(0)
	}
}

func isProjectInactive() bool {
	baseURL := strings.TrimSuffix(common.Config.Redmine.Url, "/") + "/projects.json"
	apiKey := common.Config.Redmine.Api_key
	client := &http.Client{}

	limit := 100
	offset := 0

	for {
		url := fmt.Sprintf("%s?status=5&limit=%d&offset=%d", baseURL, limit, offset)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create request to get the inactive projects from redmine for host " + common.Config.Identifier + " while uninstalling monokit.")
			return false
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Monokit/devel")
		req.Header.Set("X-Redmine-Api-Key", apiKey)

		resp, err := client.Do(req)
		if err != nil {
			log.Error().Err(err).Msg("Failed to make request to get the inactive projects from redmine for host " + common.Config.Identifier + " while uninstalling monokit.")
			return false
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			log.Error().Int("statusCode", resp.StatusCode).Str("body", string(body)).Msg("API returned error for host " + common.Config.Identifier + " while uninstalling monokit.")
			return false
		}

		var result RedmineResponse
		if err := json.Unmarshal(body, &result); err != nil {
			log.Error().Err(err).Msg("Failed to parse JSON for host " + common.Config.Identifier + " while uninstalling monokit.")
			return false
		}

		projectMap := make(map[string]bool)
		for _, p := range result.Projects {
			projectMap[p.Identifier] = true
		}

		parts := strings.Split(common.Config.Identifier, "-")
		for i := len(parts); i > 0; i-- {
			candidate := strings.Join(parts[:i], "-")
			if projectMap[candidate] {
				ProjectIdentifier = candidate
				return true
			}
		}

		if len(result.Projects) < limit {
			break
		}
		offset += limit
	}

	return false
}

func notify(message string) {
	common.Alarm(message, "alarm", ProjectIdentifier+"-self-destruct", true)
}

func removeCron() {
	output, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get crontab")
		notify("Failed to get crontab for host " + common.Config.Identifier + " while uninstalling monokit. Skipping cron removal.")
		return
	}
	var newCrontab []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "#Ansible:") {
			if scanner.Scan() {
				nextLine := scanner.Text()
				if !strings.Contains(nextLine, "/usr/local/bin/monokit") {
					newCrontab = append(newCrontab, line, nextLine)
				}
				continue
			}
			newCrontab = append(newCrontab, line)
			break
		}
		if !strings.Contains(line, "/usr/local/bin/monokit") {
			newCrontab = append(newCrontab, line)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error().Err(err).Msg("Error scanning crontab output")
	}

	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(strings.Join(newCrontab, "\n") + "\n")
	if err := cmd.Run(); err != nil {
		log.Error().Err(err).Msg("Failed to update crontab")
		notify("Failed to update crontab for host " + common.Config.Identifier)
	}
	notify("Successfully removed cron for host " + common.Config.Identifier + " while uninstalling monokit.")
}

func removeFromSSHD() {
	path := "/etc/pam.d/sshd"
	file, err := os.Open(path)
	if err != nil {
		log.Error().Err(err).Msg("Failed to open " + path)
		notify("Failed to open " + path + " for host " + common.Config.Identifier + " while uninstalling monokit.")
		return
	}
	defer file.Close()

	var newLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "monokit") {
			newLines = append(newLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error().Err(err).Msg("Error scanning " + path)
		notify("Error scanning " + path + " for host " + common.Config.Identifier + " while uninstalling monokit.")
		return
	}

	outFile, err := os.Create(path)
	if err != nil {
		log.Error().Err(err).Msg("Failed to open " + path + " for writing")
		notify("Failed to open " + path + " for writing for host " + common.Config.Identifier + " while uninstalling monokit.")
		return
	}
	defer outFile.Close()

	writer := bufio.NewWriter(outFile)
	for _, line := range newLines {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			log.Error().Err(err).Msg("Failed to write to " + path)
			notify("Failed to write to " + path + " for host " + common.Config.Identifier + " while uninstalling monokit.")
			return
		}
	}
	if err := writer.Flush(); err != nil {
		log.Error().Err(err).Msg("Failed to flush writer for " + path)
		notify("Failed to flush writer for " + path + " for host " + common.Config.Identifier + " while uninstalling monokit.")
		return
	}
	notify("Successfully removed monokit references from " + path + " for host " + common.Config.Identifier + " while uninstalling monokit.")
}

func removeShutdownNotifier() {
	cmd := exec.Command("systemctl", "list-unit-files", "shutdown-notifier.service")
	output, err := cmd.Output()
	if err != nil {
		log.Error().Err(err).Msg("Failed to check if shutdown-notifier service exists")
		notify("Failed to check if shutdown-notifier service exists for host " + common.Config.Identifier + " while uninstalling monokit.")
		return
	}

	if strings.Contains(string(output), "shutdown-notifier.service") {
		cmd = exec.Command("systemctl", "stop", "shutdown-notifier.service")
		if err := cmd.Run(); err != nil {
			log.Error().Err(err).Msg("Failed to stop shutdown-notifier service")
			notify("Failed to stop shutdown-notifier service for host " + common.Config.Identifier + " while uninstalling monokit.")
		}

		cmd = exec.Command("systemctl", "disable", "shutdown-notifier.service")
		if err := cmd.Run(); err != nil {
			log.Error().Err(err).Msg("Failed to disable shutdown-notifier service")
			notify("Failed to disable shutdown-notifier service for host " + common.Config.Identifier + " while uninstalling monokit.")
		}

		path := "/etc/systemd/system/shutdown-notifier.service"
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Error().Err(err).Msg("Failed to remove " + path)
			notify("Failed to remove " + path + " for host " + common.Config.Identifier + " while uninstalling monokit.")
		} else {
			notify("Successfully removed shutdown-notifier service for host " + common.Config.Identifier + " while uninstalling monokit.")
		}

		cmd = exec.Command("systemctl", "daemon-reload")
		if err := cmd.Run(); err != nil {
			log.Error().Err(err).Msg("Failed to reload daemon")
		}
	}
}

func deleteMonokit() {
	xdgStateHome := os.Getenv("XDG_STATE_HOME")
	if xdgStateHome == "" {
		xdgStateHome = os.Getenv("HOME") + "/.local/state"
	}
	paths := []string{
		"/var/lib/monokit",
		"/etc/mono",
		"/etc/mono.sh",
		"/tmp/mono",
		"/tmp/monokit",
		"/tmp/mono.sh",
		"/var/log/monokit",
		"/var/log/monokit.log",
		xdgStateHome + "/monokit",
		"/usr/local/bin/monokit",
	}
	for _, path := range paths {
		exists, err := exists(path)
		if err != nil {
			log.Error().Err(err).Msg("Failed to check if " + path + " exists")
			notify("Failed to check if " + path + " exists for host " + common.Config.Identifier + " while uninstalling monokit.")
			return
		}
		if exists {
			err = os.RemoveAll(path)
			if err != nil {
				log.Error().Err(err).Msg("Failed to remove " + path)
				notify("Failed to remove " + path + " for host " + common.Config.Identifier + " while uninstalling monokit.")
			} else {
				notify("Successfully removed " + path + " for host " + common.Config.Identifier + " while uninstalling monokit.")
			}
		}
	}
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

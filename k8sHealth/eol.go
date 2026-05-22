//go:build plugin

package k8sHealth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common/healthdb"
	"github.com/rs/zerolog/log"
)

const (
	eolEndpointURL    = "https://endoflife.date/api/kubernetes.json"
	eolHTTPTimeout    = 10 * time.Second
	eolCacheTTL       = 24 * time.Hour
	eolHealthdbModule = "k8s_eol"
	eolHealthdbKey    = "kubernetes"
	eolAlarmService   = "kubernetes_eol"
	defaultWarnDays   = 180
)

type eolCycle struct {
	Cycle             string      `json:"cycle"`
	ReleaseDate       string      `json:"releaseDate"`
	EOL               string      `json:"eol"`
	Latest            string      `json:"latest"`
	LatestReleaseDate string      `json:"latestReleaseDate"`
	LTS               interface{} `json:"lts"`
	Support           interface{} `json:"support"`
}

type eolCacheEntry struct {
	FetchedAt time.Time  `json:"fetchedAt"`
	Cycles    []eolCycle `json:"cycles"`
}

func isKubernetesEOLEnabled() bool {
	if K8sHealthConfig.EOL.Enabled != nil {
		return *K8sHealthConfig.EOL.Enabled
	}
	return true
}

func kubernetesEOLWarnDays() int {
	if K8sHealthConfig.EOL.WarnDays != nil && *K8sHealthConfig.EOL.WarnDays >= 0 {
		return *K8sHealthConfig.EOL.WarnDays
	}
	return defaultWarnDays
}

// parseKubernetesVersion strips leading "v", drops any "+suffix" (e.g. "+rke2r1")
// and any "-prerelease" tag, returning the bare semantic version like "1.34.6".
func parseKubernetesVersion(raw string) string {
	v := strings.TrimSpace(raw)
	v = strings.TrimPrefix(v, "v")
	if idx := strings.IndexAny(v, "+-"); idx > 0 {
		v = v[:idx]
	}
	return v
}

// extractMinorCycle returns the "major.minor" portion of a semver string,
// matching the cycle naming used by endoflife.date (e.g. "1.34").
func extractMinorCycle(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return ""
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return ""
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return ""
	}
	return parts[0] + "." + parts[1]
}

func fetchKubernetesEOLCycles() ([]eolCycle, error) {
	cachedJSON, cachedAt, _, found, _ := healthdb.GetJSON(eolHealthdbModule, eolHealthdbKey)
	if found && cachedJSON != "" && time.Since(cachedAt) < eolCacheTTL {
		var entry eolCacheEntry
		if err := json.Unmarshal([]byte(cachedJSON), &entry); err == nil && len(entry.Cycles) > 0 {
			return entry.Cycles, nil
		}
	}

	client := &http.Client{Timeout: eolHTTPTimeout}
	req, err := http.NewRequest("GET", eolEndpointURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Monokit-k8sHealth/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch endoflife.date: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("endoflife.date returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read endoflife.date body: %w", err)
	}

	var cycles []eolCycle
	if err := json.Unmarshal(body, &cycles); err != nil {
		return nil, fmt.Errorf("failed to parse endoflife.date response: %w", err)
	}

	entry := eolCacheEntry{FetchedAt: time.Now(), Cycles: cycles}
	if b, err := json.Marshal(entry); err == nil {
		_ = healthdb.PutJSON(eolHealthdbModule, eolHealthdbKey, string(b), nil, time.Now())
	}

	return cycles, nil
}

func parseEOLDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// CollectKubernetesEOL gathers Kubernetes End-of-Life information for the
// running server version and emits alarms via the standard k8sHealth alarm
// helpers. Result is always non-nil.
func CollectKubernetesEOL() *KubernetesEOLInfo {
	info := &KubernetesEOLInfo{WarnDays: kubernetesEOLWarnDays()}

	if !isKubernetesEOLEnabled() {
		info.Skipped = true
		info.SkipReason = "EOL check disabled via config (eol.enabled=false)"
		return info
	}

	versionInfo, err := GetKubernetesServerVersion()
	if err != nil {
		info.Error = fmt.Sprintf("failed to query Kubernetes server version: %v", err)
		log.Error().
			Str("component", "k8sHealth").
			Str("operation", "collect_kubernetes_eol").
			Err(err).
			Msg("Could not fetch Kubernetes server version for EOL check")
		return info
	}
	info.RawVersion = versionInfo.GitVersion
	info.CurrentVersion = parseKubernetesVersion(versionInfo.GitVersion)
	info.Cycle = extractMinorCycle(info.CurrentVersion)
	if info.Cycle == "" {
		info.Error = fmt.Sprintf("could not parse Kubernetes version %q into a major.minor cycle", info.RawVersion)
		return info
	}

	cycles, err := fetchKubernetesEOLCycles()
	if err != nil {
		info.Error = err.Error()
		log.Warn().
			Str("component", "k8sHealth").
			Str("operation", "collect_kubernetes_eol").
			Err(err).
			Msg("Could not fetch Kubernetes EOL data")
		return info
	}

	var matched *eolCycle
	for i := range cycles {
		if cycles[i].Cycle == info.Cycle {
			matched = &cycles[i]
			break
		}
	}
	if matched == nil {
		info.Error = fmt.Sprintf("no matching cycle %q on endoflife.date", info.Cycle)
		log.Warn().
			Str("component", "k8sHealth").
			Str("operation", "collect_kubernetes_eol").
			Str("cycle", info.Cycle).
			Msg("Kubernetes cycle not found on endoflife.date")
		return info
	}

	info.LatestInCycle = matched.Latest
	if d, ok := parseEOLDate(matched.EOL); ok {
		info.EOLDate = d
	}
	if s, ok := matched.Support.(string); ok {
		if d, ok2 := parseEOLDate(s); ok2 {
			info.SupportDate = d
		}
	}
	info.Checked = true

	if info.EOLDate.IsZero() {
		info.Error = "endoflife.date returned no EOL date for this cycle"
		return info
	}

	now := time.Now()
	info.DaysUntilEOL = int(info.EOLDate.Sub(now).Hours() / 24)
	info.IsEOL = !now.Before(info.EOLDate)
	info.IsNearEOL = !info.IsEOL && info.DaysUntilEOL <= info.WarnDays

	switch {
	case info.IsEOL:
		msg := fmt.Sprintf(
			"Kubernetes %s (cycle %s) is past End-of-Life (%s). Latest in cycle: %s. Upgrade required.",
			info.CurrentVersion, info.Cycle, info.EOLDate.Format("2006-01-02"), info.LatestInCycle,
		)
		alarmCheckDown(eolAlarmService, msg, false, "", "")
	case info.IsNearEOL:
		msg := fmt.Sprintf(
			"Kubernetes %s (cycle %s) reaches End-of-Life in %d days (%s). Latest in cycle: %s. Plan upgrade.",
			info.CurrentVersion, info.Cycle, info.DaysUntilEOL, info.EOLDate.Format("2006-01-02"), info.LatestInCycle,
		)
		alarmCheckDown(eolAlarmService, msg, false, "", "")
	default:
		msg := fmt.Sprintf(
			"Kubernetes %s (cycle %s) supported until %s (%d days).",
			info.CurrentVersion, info.Cycle, info.EOLDate.Format("2006-01-02"), info.DaysUntilEOL,
		)
		alarmCheckUp(eolAlarmService, msg, false)
	}

	log.Debug().
		Str("component", "k8sHealth").
		Str("operation", "collect_kubernetes_eol").
		Str("current_version", info.CurrentVersion).
		Str("cycle", info.Cycle).
		Str("latest_in_cycle", info.LatestInCycle).
		Time("eol_date", info.EOLDate).
		Int("days_until_eol", info.DaysUntilEOL).
		Bool("is_eol", info.IsEOL).
		Bool("is_near_eol", info.IsNearEOL).
		Msg("Kubernetes EOL check completed")

	return info
}

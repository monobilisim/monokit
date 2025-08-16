//go:build linux

package upCheck

import (
    "fmt"
    "net/http"
    "strings"
    "time"

    "github.com/monobilisim/monokit/common"
    issues "github.com/monobilisim/monokit/common/redmine/issues"
    "github.com/rs/zerolog/log"
    "github.com/spf13/cobra"
)

// Config represents the YAML configuration for upCheck
type Config struct {
    Services []string   `yaml:"services"`
    URLs     []URLCheck `yaml:"urls"`
}

var UpCheckConfig Config

// URLCheck describes a single URL to check
type URLCheck struct {
    URL           string `yaml:"url"`
    Expected      int    `yaml:"expected"`
    TimeoutSecond int    `yaml:"timeout_seconds"`
    Method        string `yaml:"method"`
}

// Main is the cobra run function for the upCheck command
func Main(cmd *cobra.Command, args []string) {
    version := "1.0.0"
    _ = version // reserved for future use in UI output

    common.ScriptName = "upCheck"
    common.TmpDir = common.TmpDir + "upCheck"
    common.Init()

    // Load config from /etc/mono/upcheck.yaml if present
    if common.ConfExists("upcheck") {
        common.ConfInit("upcheck", &UpCheckConfig)
    } else {
        log.Warn().Msg("upCheck: /etc/mono/upcheck.yaml bulunamadı; örnek için repo içindeki config/upcheck.yaml dosyasına bakın")
    }

    // Check service status with the Monokit server (enable/disable, updates)
    common.WrapperGetServiceStatus("upCheck")

    if len(UpCheckConfig.Services) == 0 && len(UpCheckConfig.URLs) == 0 {
        log.Warn().Msg("upCheck: no checks configured under 'services' or 'urls' in /etc/mono/upcheck.yaml")
        fmt.Println("No checks configured. Add a services or urls list to /etc/mono/upcheck.yaml")
        return
    }

    var outputBuilder strings.Builder
    outputBuilder.WriteString("monokit upCheck\n\n")

    for _, unit := range UpCheckConfig.Services {
        unit = strings.TrimSpace(unit)
        if unit == "" {
            continue
        }

        // Build keys/messages
        alarmKey := "upcheck_" + sanitizeKey(unit)
        redmineServiceKey := "upcheck/" + unit

        // Determine state
        exists := common.SystemdUnitExists(unit)
        active := common.SystemdUnitActive(unit)

        // Prepare human output
        status := "DOWN"
        if active {
            status = "UP"
        }
        if !exists {
            status = "MISSING"
        }
        outputBuilder.WriteString(fmt.Sprintf("- %s: %s\n", unit, status))

        // Alarm + Redmine logic
        if !exists || !active {
            // Turkish subject/body for Redmine
            subject := common.Config.Identifier + " için " + unit + " servisi çalışmıyor"
            reason := "servis aktif değil"
            if !exists {
                reason = "servis bulunamadı"
            }
            body := "Hedef sunucu: " + common.Config.Identifier + "\n" +
                "Servis: " + unit + "\n" +
                "Durum: " + strings.ToUpper(reason) + "\n" +
                "Kontrol: systemctl status " + unit + "\n"

            // English alarm messages aligned with other tools
            var alarmMsg string
            if !exists {
                alarmMsg = unit + " service is not installed"
            } else {
                alarmMsg = unit + " service is not running"
            }

            // Send alarm and ensure/update Redmine issue
            common.AlarmCheckDown(alarmKey, alarmMsg, false, "", "")
            issues.CheckDown(redmineServiceKey, subject, body, false, 0)
        } else {
            // Recovery alarm and close Redmine issue
            upMsg := unit + " service is now running"
            common.AlarmCheckUp(alarmKey, upMsg, false)
            issues.CheckUp(redmineServiceKey, common.Config.Identifier+" için "+unit+" servisi yeniden çalışıyor, kapatılıyor.")
        }
    }

    // URL checks
    if len(UpCheckConfig.URLs) > 0 {
        httpClient := &http.Client{Timeout: 5 * time.Second}
        for _, u := range UpCheckConfig.URLs {
            urlStr := strings.TrimSpace(u.URL)
            if urlStr == "" {
                continue
            }
            expected := u.Expected
            if expected == 0 {
                expected = http.StatusOK
            }
            timeout := u.TimeoutSecond
            if timeout <= 0 {
                timeout = 5
            }
            method := strings.ToUpper(strings.TrimSpace(u.Method))
            if method == "" {
                method = http.MethodGet
            }

            // Prepare request
            req, err := http.NewRequest(method, urlStr, nil)
            if err != nil {
                // Treat as failure
                writeURLFailure(&outputBuilder, urlStr, expected, 0)
                alarmKey := "upcheck_url_" + sanitizeKey(urlStr)
                subject := common.Config.Identifier + " için URL çalışmıyor"
                body := "Hedef: " + common.Config.Identifier + "\n" +
                    "URL: " + urlStr + "\n" +
                    "Durum: İSTEK OLUŞTURULAMADI\n"
                common.AlarmCheckDown(alarmKey, "URL check failed: request could not be created", false, "", "")
                issues.CheckDown("upcheck/url:"+urlStr, subject, body, false, 0)
                continue
            }
            // Adjust timeout per URL
            httpClient.Timeout = time.Duration(timeout) * time.Second
            common.AddUserAgent(req)

            resp, err := httpClient.Do(req)
            if err != nil {
                writeURLFailure(&outputBuilder, urlStr, expected, 0)
                alarmKey := "upcheck_url_" + sanitizeKey(urlStr)
                subject := common.Config.Identifier + " için URL çalışmıyor"
                body := "Hedef: " + common.Config.Identifier + "\n" +
                    "URL: " + urlStr + "\n" +
                    "Durum: ERİŞİLEMEDİ (zaman aşımı ya da ağ hatası)\n"
                common.AlarmCheckDown(alarmKey, "URL check failed: unreachable (timeout or network error)", false, "", "")
                issues.CheckDown("upcheck/url:"+urlStr, subject, body, false, 0)
                continue
            }

            actual := resp.StatusCode
            _ = resp.Body.Close()

            if actual != expected {
                writeURLFailure(&outputBuilder, urlStr, expected, actual)
                alarmKey := "upcheck_url_" + sanitizeKey(urlStr)
                subject := common.Config.Identifier + " için URL beklenen kodu döndürmüyor"
                body := "Hedef: " + common.Config.Identifier + "\n" +
                    "URL: " + urlStr + "\n" +
                    fmt.Sprintf("Beklenen: %d\n", expected) +
                    fmt.Sprintf("Gelen: %d\n", actual)
                common.AlarmCheckDown(alarmKey, fmt.Sprintf("URL check failed: unexpected status (expected %d, got %d)", expected, actual), false, "", "")
                issues.CheckDown("upcheck/url:"+urlStr, subject, body, false, 0)
            } else {
                writeURLSuccess(&outputBuilder, urlStr, expected)
                alarmKey := "upcheck_url_" + sanitizeKey(urlStr)
                okMsg := fmt.Sprintf("URL returned expected status %d: %s", expected, urlStr)
                common.AlarmCheckUp(alarmKey, okMsg, false)
                issues.CheckUp("upcheck/url:"+urlStr, common.Config.Identifier+" için URL beklenen kodu döndürüyor, kapatılıyor.")
            }
        }
    }

    fmt.Println(outputBuilder.String())
}

func sanitizeKey(s string) string {
    // Map any non-alphanumeric character to '-'
    var b strings.Builder
    for _, r := range s {
        if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
            b.WriteRune(r)
        } else {
            b.WriteRune('-')
        }
    }
    return b.String()
}

func writeURLFailure(sb *strings.Builder, urlStr string, expected, actual int) {
    if actual == 0 {
        sb.WriteString(fmt.Sprintf("- URL %s: DOWN (beklenen %d, gelen -)\n", urlStr, expected))
        return
    }
    sb.WriteString(fmt.Sprintf("- URL %s: DOWN (beklenen %d, gelen %d)\n", urlStr, expected, actual))
}

func writeURLSuccess(sb *strings.Builder, urlStr string, expected int) {
    sb.WriteString(fmt.Sprintf("- URL %s: UP (beklenen %d)\n", urlStr, expected))
}

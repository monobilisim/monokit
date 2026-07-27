//go:build plugin

package k8sHealth

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	issues "github.com/monobilisim/monokit/common/redmine/issues"
	"github.com/rs/zerolog/log"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const defaultKubeconfigCertWarnDays = 30

func isKubeconfigCertCheckEnabled() bool {
	if K8sHealthConfig.KubeconfigCert.Enabled != nil {
		return *K8sHealthConfig.KubeconfigCert.Enabled
	}
	return true
}

func kubeconfigCertWarnDays() int {
	if K8sHealthConfig.KubeconfigCert.WarnDays != nil && *K8sHealthConfig.KubeconfigCert.WarnDays >= 0 {
		return *K8sHealthConfig.KubeconfigCert.WarnDays
	}
	return defaultKubeconfigCertWarnDays
}

// sanitizeAlarmKeyPart makes a kubeconfig user name (e.g. "system:node:worker1")
// safe to embed in an alarm/issue service key.
func sanitizeAlarmKeyPart(s string) string {
	replacer := strings.NewReplacer(":", "_", "/", "-", " ", "_")
	return replacer.Replace(s)
}

// CollectKubeconfigCertHealth checks the expiry of the client certificate(s)
// embedded in (or referenced by) the local kubeconfig file — i.e. the
// certificate monokit/kubectl authenticates to the API server with, as
// opposed to the API server's own serving certificate (see
// CollectClusterApiCertHealth). These client certs are typically
// auto-renewed and redistributed to other masters on install, but static
// integrations (Uptime Kuma, GitLab, ...) configured with a copy of them
// need manual updates before they expire. Result is always non-nil.
func CollectKubeconfigCertHealth() *KubeconfigCertHealth {
	info := &KubeconfigCertHealth{WarnDays: kubeconfigCertWarnDays()}

	if !isKubeconfigCertCheckEnabled() {
		info.Skipped = true
		info.SkipReason = "kubeconfig client certificate check disabled via config (kubeconfig_cert.enabled=false)"
		return info
	}

	kubeconfigPath := GetKubeconfigPath("")
	if kubeconfigPath == "" {
		info.Skipped = true
		info.SkipReason = "no kubeconfig file found on this host"
		return info
	}
	info.KubeconfigPath = kubeconfigPath

	cfg, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		info.Error = fmt.Sprintf("failed to load kubeconfig %s: %v", kubeconfigPath, err)
		log.Error().
			Str("component", "k8sHealth").
			Str("operation", "collect_kubeconfig_cert_health").
			Err(err).
			Msg("Failed to load kubeconfig file")
		alarmCheckDown("kubeconfig_client_cert_read", info.Error, false, "", "")
		return info
	}
	alarmCheckUp("kubeconfig_client_cert_read", fmt.Sprintf("Kubeconfig file read successfully: %s", kubeconfigPath), false)

	names := make([]string, 0, len(cfg.AuthInfos))
	for name := range cfg.AuthInfos {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		entry := collectKubeconfigCertEntry(kubeconfigPath, name, cfg.AuthInfos[name], info.WarnDays)
		if entry != nil {
			info.Entries = append(info.Entries, *entry)
		}
	}

	info.Checked = true
	return info
}

// collectKubeconfigCertEntry parses the client certificate for a single
// kubeconfig "user" entry and raises the corresponding alarm/Redmine issue.
// Returns nil if the auth entry has no client certificate at all (e.g. it
// authenticates via token or exec plugin instead).
func collectKubeconfigCertEntry(kubeconfigPath, name string, authInfo *clientcmdapi.AuthInfo, warnDays int) *KubeconfigCertEntry {
	entry := &KubeconfigCertEntry{UserName: name}
	key := "kubeconfig_client_cert_expiry_" + sanitizeAlarmKeyPart(name)

	var certBytes []byte
	var err error
	switch {
	case len(authInfo.ClientCertificateData) > 0:
		certBytes = authInfo.ClientCertificateData
	case authInfo.ClientCertificate != "":
		certBytes, err = os.ReadFile(authInfo.ClientCertificate)
		if err != nil {
			entry.Error = fmt.Sprintf("failed to read client certificate file %s: %v", authInfo.ClientCertificate, err)
			log.Error().
				Str("component", "k8sHealth").
				Str("operation", "collect_kubeconfig_cert_health").
				Str("user", name).
				Err(err).
				Msg("Failed to read client certificate file")
			alarmCheckDown(key, entry.Error, false, "", "")
			return entry
		}
	default:
		return nil
	}

	block, _ := pem.Decode(certBytes)
	if block == nil {
		entry.Error = fmt.Sprintf("failed to parse PEM block from client certificate for kubeconfig user %q", name)
		alarmCheckDown(key, entry.Error, false, "", "")
		return entry
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		entry.Error = fmt.Sprintf("failed to parse client certificate for kubeconfig user %q: %v", name, err)
		alarmCheckDown(key, entry.Error, false, "", "")
		return entry
	}

	entry.NotAfter = cert.NotAfter
	entry.DaysUntilExpiry = int(time.Until(cert.NotAfter).Hours() / 24)
	entry.IsExpired = !cert.NotAfter.After(time.Now())
	entry.IsExpiringSoon = !entry.IsExpired && entry.DaysUntilExpiry <= warnDays

	switch {
	case entry.IsExpired:
		alarmMsg := fmt.Sprintf("Kubeconfig client certificate for user %q is EXPIRED. Expired: %s", name, entry.NotAfter.Format(time.RFC3339))
		alarmCheckDown(key, alarmMsg, false, "", "")
		subject := fmt.Sprintf("%s sunucusunun kubeconfig client sertifikasının (%s) süresi doldu", common.Config.Identifier, name)
		body := fmt.Sprintf(
			"Kubeconfig dosyası: %s\nKullanıcı: %s\nSon kullanma tarihi: %s\n\nBu sertifikanın süresi doldu. Sertifika otomatik olarak yenilenip diğer masterlara dağıtılıyor olsa da, Uptime Kuma, GitLab gibi statik olarak sertifika girilen entegrasyonların elle güncellenmesi gerekiyor.",
			kubeconfigPath, name, entry.NotAfter.Format("2006-01-02"),
		)
		issues.CheckDown(key, subject, body, false, 0)
	case entry.IsExpiringSoon:
		alarmMsg := fmt.Sprintf("Kubeconfig client certificate for user %q expires in %d day(s) (%s).", name, entry.DaysUntilExpiry, entry.NotAfter.Format(time.RFC3339))
		alarmCheckDown(key, alarmMsg, false, "", "")
		subject := fmt.Sprintf("%s sunucusunun kubeconfig client sertifikası (%s) bitimine %d gün kaldı", common.Config.Identifier, name, entry.DaysUntilExpiry)
		body := fmt.Sprintf(
			"Kubeconfig dosyası: %s\nKullanıcı: %s\nSon kullanma tarihi: %s (%d gün kaldı)\n\nBu sertifika otomatik olarak yenilenip diğer masterlara dağıtılıyor, ancak Uptime Kuma, GitLab gibi statik olarak sertifika girilen entegrasyonların da elle güncellenmesi gerekiyor.",
			kubeconfigPath, name, entry.NotAfter.Format("2006-01-02"), entry.DaysUntilExpiry,
		)
		issues.CheckDown(key, subject, body, false, 0)
	default:
		alarmMsg := fmt.Sprintf("Kubeconfig client certificate for user %q is valid. Expires: %s (%d day(s)).", name, entry.NotAfter.Format(time.RFC3339), entry.DaysUntilExpiry)
		alarmCheckUp(key, alarmMsg, false)
		issues.CheckUp(key, fmt.Sprintf("Kubeconfig client sertifikası (%s) artık %d gün sonra sona erecek şekilde güncellendi", name, entry.DaysUntilExpiry))
	}

	return entry
}

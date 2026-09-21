package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
)

// helmRelease mirrors the releaseElement struct that `helm list --output json`
// emits. The shape is identical in Helm 3 (cmd/helm/list.go) and Helm 4
// (pkg/cmd/list.go), so one parser covers both majors.
type helmRelease struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Revision   string `json:"revision"`
	Updated    string `json:"updated"`
	Status     string `json:"status"`
	Chart      string `json:"chart"`
	AppVersion string `json:"app_version"`
}

// helmChartRegex splits a chart reference such as "mimir-distributed-5.4.0" into
// name and version. Chart names contain dashes and digits themselves, so the
// greedy prefix forces the split at the last plausible version boundary:
// "foo-1.2.3-beta.1" splits after "foo", not after "foo-1.2.3".
var helmChartRegex = regexp.MustCompile(`^(.*)-(v?\d+\.\d+\.\d+.*)$`)

// VersionCheckConfig holds the optional /etc/mono/versioncheck.yml settings.
var VersionCheckConfig struct {
	Helm struct {
		Enabled    bool     `yaml:"enabled"`
		Kubeconfig string   `yaml:"kubeconfig"`
		Namespaces []string `yaml:"namespaces"`
	} `yaml:"helm"`
}

// helmList runs `helm list` over the given scope (either --all-namespaces or a
// --namespace flag) and decodes the result.
func helmList(scope ...string) ([]helmRelease, error) {
	args := append([]string{"list"}, scope...)
	args = append(args, "--output", "json")

	cmd := exec.Command("helm", args...)

	// Under cron the invoking user usually has no ~/.kube/config, so the
	// kubeconfig that the cluster actually uses has to be pointed at explicitly
	// (e.g. /etc/rancher/rke2/rke2.yaml on RKE2, /etc/rancher/k3s/k3s.yaml on k3s).
	if kubeconfig := VersionCheckConfig.Helm.Kubeconfig; kubeconfig != "" {
		cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfig)
	}

	out, err := cmd.Output()
	if err != nil {
		// helm explains RBAC and connection failures on stderr; without it the
		// recorded error is just "exit status 1".
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}

	var releases []helmRelease
	if err := json.Unmarshal(out, &releases); err != nil {
		return nil, fmt.Errorf("could not parse helm list output: %w", err)
	}

	return releases, nil
}

// HelmCheck records the chart version of every deployed Helm release.
//
// Helm releases are cluster-wide while versionCheck runs on every node, so this
// check is opt-in: set helm.enabled in /etc/mono/versioncheck.yml on exactly one
// node per cluster. Enabling it on each control-plane node would file one
// duplicate Redmine news item per node for the same chart upgrade.
func HelmCheck() {
	if !common.ConfExists("versioncheck") {
		log.Debug().Msg("No versioncheck config found, skipping Helm check")
		return
	}

	common.ConfInit("versioncheck", &VersionCheckConfig)

	if !VersionCheckConfig.Helm.Enabled {
		log.Debug().Msg("Helm check not enabled in versioncheck config, skipping")
		return
	}

	if _, err := exec.LookPath("helm"); err != nil {
		log.Debug().Msg("helm binary not found, skipping Helm check")
		addToNotInstalled("Helm")
		return
	}

	var releases []helmRelease

	if namespaces := helmNamespaces(); len(namespaces) > 0 {
		// Listing each namespace separately rather than filtering the
		// cluster-wide output keeps the check working where monokit's
		// kubeconfig may only read those namespaces: --all-namespaces needs
		// cluster-wide permission to list secrets and would fail outright.
		for _, namespace := range namespaces {
			found, err := helmList("--namespace", namespace)
			if err != nil {
				// One unreadable namespace should not hide the others.
				log.Error().Err(err).Str("namespace", namespace).Msg("Error listing Helm releases")
				addToVersionErrors(fmt.Errorf("Error listing Helm releases in namespace %s: %s", namespace, err.Error()))
				continue
			}
			releases = append(releases, found...)
		}
	} else {
		found, err := helmList("--all-namespaces")
		if err != nil {
			log.Error().Err(err).Msg("Error listing Helm releases")
			addToVersionErrors(fmt.Errorf("Error listing Helm releases: %s", err.Error()))
			return
		}
		releases = found
	}

	if len(releases) == 0 {
		log.Debug().Msg("No Helm releases found")
		return
	}

	for _, release := range releases {
		// A release caught mid-upgrade reports the chart it is moving to, which
		// would flap the stored version if the upgrade then fails or rolls back.
		if !strings.EqualFold(release.Status, "deployed") {
			log.Debug().Str("release", release.Name).Str("status", release.Status).Msg("Skipping Helm release that is not deployed")
			continue
		}

		m := helmChartRegex.FindStringSubmatch(release.Chart)
		if len(m) != 3 {
			log.Debug().Str("release", release.Name).Str("chart", release.Chart).Msg("Could not split Helm chart name and version")
			continue
		}
		chartName, chartVersion := m[1], m[2]

		// Namespace-qualified: the same chart is frequently released more than
		// once in a cluster, so the release name alone is not a unique key.
		name := fmt.Sprintf("Helm: %s (%s/%s)", chartName, release.Namespace, release.Name)
		key := fmt.Sprintf("helm-%s-%s", release.Namespace, release.Name)

		recordVersion(name, key, chartVersion)
	}
}

// helmNamespaces returns the configured namespace filter, dropping blank entries
// so a half-filled YAML list does not turn into a `--namespace ""` lookup. An
// empty result means "every namespace".
func helmNamespaces() []string {
	var namespaces []string

	for _, namespace := range VersionCheckConfig.Helm.Namespaces {
		if namespace = strings.TrimSpace(namespace); namespace != "" {
			namespaces = append(namespaces, namespace)
		}
	}

	return namespaces
}

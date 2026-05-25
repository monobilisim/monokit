//go:build plugin

package k8sHealth

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	rke2EtcdSnapshotDir = "/var/lib/rancher/rke2/server/db/snapshots"
	rke2EtcdctlPath     = "/var/lib/rancher/rke2/bin/etcdctl"
	defaultMaxAgeHours  = 25
)

func shouldCollectEtcdBackup() bool {
	if K8sHealthConfig.EtcdBackup.Enabled != nil {
		return *K8sHealthConfig.EtcdBackup.Enabled
	}
	return isRKE2Environment()
}

func getEtcdBackupMaxAgeHours() int {
	if K8sHealthConfig.EtcdBackup.MaxAgeHours != nil && *K8sHealthConfig.EtcdBackup.MaxAgeHours > 0 {
		return *K8sHealthConfig.EtcdBackup.MaxAgeHours
	}
	return defaultMaxAgeHours
}

type etcdctlSnapshotStatus struct {
	Hash      uint32 `json:"hash"`
	Revision  int64  `json:"revision"`
	TotalKey  int    `json:"totalKey"`
	TotalSize int64  `json:"totalSize"`
}

func verifySnapshotWithEtcdctl(etcdctlBin, snapshotPath string) (*etcdctlSnapshotStatus, error) {
	var cmd *exec.Cmd
	if strings.HasSuffix(etcdctlBin, "etcdutl") {
		cmd = exec.Command(etcdctlBin, "snapshot", "status", snapshotPath, "--write-out=json")
	} else {
		cmd = exec.Command(etcdctlBin, "snapshot", "status", snapshotPath, "--write-out=json")
		cmd.Env = append(os.Environ(), "ETCDCTL_API=3")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("etcdctl snapshot status failed: %w (output: %s)", err, string(output))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var jsonLine string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{") {
			jsonLine = trimmed
			break
		}
	}
	if jsonLine == "" {
		return nil, fmt.Errorf("no JSON found in etcdctl output: %s", string(output))
	}

	var status etcdctlSnapshotStatus
	if err := json.Unmarshal([]byte(jsonLine), &status); err != nil {
		return nil, fmt.Errorf("failed to parse etcdctl output: %w", err)
	}

	return &status, nil
}

// BoltDB page layout: 16-byte page header, then 4-byte magic at offset 16.
// Magic 0xED0CDAED in little-endian = {0xED, 0xDA, 0x0C, 0xED}.
const boltDBMagicOffset = 16

var boltDBMagic = []byte{0xED, 0xDA, 0x0C, 0xED}

func validateBoltDBHeader(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, boltDBMagicOffset+4)
	n, err := f.Read(buf)
	if err != nil || n < boltDBMagicOffset+4 {
		return false, fmt.Errorf("cannot read header: file too small or unreadable")
	}

	for i := 0; i < 4; i++ {
		if buf[boltDBMagicOffset+i] != boltDBMagic[i] {
			return false, nil
		}
	}
	return true, nil
}

func findEtcdctlBinary() string {
	if _, err := os.Stat(rke2EtcdctlPath); err == nil {
		return rke2EtcdctlPath
	}

	containerdDir := "/var/lib/rancher/rke2/data"
	if entries, err := os.ReadDir(containerdDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			candidate := filepath.Join(containerdDir, entry.Name(), "bin", "etcdctl")
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	matches, _ := filepath.Glob("/var/lib/rancher/rke2/agent/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/*/fs/usr/local/bin/etcdctl")
	if len(matches) > 0 {
		return matches[0]
	}

	if path, err := exec.LookPath("etcdutl"); err == nil {
		return path
	}

	if path, err := exec.LookPath("etcdctl"); err == nil {
		return path
	}

	return ""
}

func isSnapshotFile(name string) bool {
	if strings.HasSuffix(name, ".tmp") || strings.HasPrefix(name, ".") {
		return false
	}
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".gz") ||
		strings.HasSuffix(lower, ".db") || strings.Contains(lower, "etcd-snapshot") ||
		strings.Contains(lower, "on-demand") {
		return true
	}
	if !strings.Contains(name, ".") {
		return true
	}
	return false
}

func CollectEtcdBackupHealth() *EtcdBackupHealth {
	health := &EtcdBackupHealth{
		SnapshotDir: rke2EtcdSnapshotDir,
		MaxAgeHours: getEtcdBackupMaxAgeHours(),
	}

	if !isRKE2Environment() {
		health.Skipped = true
		health.SkipReason = "Not an RKE2 environment"
		log.Debug().
			Str("component", "k8sHealth").
			Str("operation", "collect_etcd_backup_health").
			Msg("Skipping etcd backup check: not RKE2")
		return health
	}

	if _, err := os.Stat(rke2EtcdSnapshotDir); os.IsNotExist(err) {
		health.Checked = true
		health.Error = fmt.Sprintf("Snapshot directory not found: %s", rke2EtcdSnapshotDir)
		alarmCheckDown("etcd_backup_dir", health.Error, false, "", "")
		log.Warn().
			Str("component", "k8sHealth").
			Str("operation", "collect_etcd_backup_health").
			Str("snapshot_dir", rke2EtcdSnapshotDir).
			Msg("etcd snapshot directory not found")
		return health
	}

	entries, err := os.ReadDir(rke2EtcdSnapshotDir)
	if err != nil {
		health.Checked = true
		health.Error = fmt.Sprintf("Failed to read snapshot directory: %v", err)
		alarmCheckDown("etcd_backup_dir_read", health.Error, false, "", "")
		return health
	}

	etcdctlBin := findEtcdctlBinary()
	hasEtcdctl := etcdctlBin != ""
	if hasEtcdctl {
		log.Debug().
			Str("component", "k8sHealth").
			Str("operation", "collect_etcd_backup_health").
			Str("etcdctl_path", etcdctlBin).
			Msg("Found etcdctl binary for snapshot verification")
	}

	var snapshots []EtcdSnapshotInfo

	for _, entry := range entries {
		if entry.IsDir() || !isSnapshotFile(entry.Name()) {
			continue
		}

		snapshotPath := filepath.Join(rke2EtcdSnapshotDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			snapshots = append(snapshots, EtcdSnapshotInfo{
				Filename: entry.Name(),
				Path:     snapshotPath,
				IsValid:  false,
				Error:    fmt.Sprintf("Failed to get file info: %v", err),
			})
			continue
		}

		snap := EtcdSnapshotInfo{
			Filename: entry.Name(),
			Path:     snapshotPath,
			Size:     info.Size(),
			ModTime:  info.ModTime(),
			IsValid:  true,
		}

		if info.Size() == 0 {
			snap.IsValid = false
			snap.Error = "Snapshot file is empty (0 bytes)"
		} else if !isCompressedSnapshot(entry.Name()) {
			isBoltDB, err := validateBoltDBHeader(snapshotPath)
			if err != nil {
				log.Warn().
					Str("component", "k8sHealth").
					Str("operation", "collect_etcd_backup_health").
					Str("snapshot", entry.Name()).
					Err(err).
					Msg("Failed to validate BoltDB header")
			} else if !isBoltDB {
				snap.IsValid = false
				snap.Error = "File is not a valid BoltDB database (bad magic bytes)"
			}

			if snap.IsValid && hasEtcdctl {
				status, err := verifySnapshotWithEtcdctl(etcdctlBin, snapshotPath)
				if err != nil {
					log.Warn().
						Str("component", "k8sHealth").
						Str("operation", "collect_etcd_backup_health").
						Str("snapshot", entry.Name()).
						Err(err).
						Msg("etcdctl verification failed, file passed BoltDB header check")
				} else {
					snap.Hash = fmt.Sprintf("%x", status.Hash)
					snap.Revision = status.Revision
					snap.TotalKey = int64(status.TotalKey)
					if status.TotalKey == 0 {
						snap.IsValid = false
						snap.Error = "Snapshot has 0 keys"
					}
				}
			}
		}

		snapshots = append(snapshots, snap)
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].ModTime.After(snapshots[j].ModTime)
	})

	health.Checked = true
	health.Snapshots = snapshots
	health.TotalSnapshots = len(snapshots)

	for _, s := range snapshots {
		if s.IsValid {
			health.ValidSnapshots++
		} else {
			health.InvalidSnapshots++
		}
	}

	if len(snapshots) > 0 {
		health.LatestSnapshot = &snapshots[0]
		health.OldestSnapshot = &snapshots[len(snapshots)-1]

		age := time.Since(health.LatestSnapshot.ModTime)
		maxAge := time.Duration(health.MaxAgeHours) * time.Hour
		health.IsLatestTooOld = age > maxAge
	}

	alarmKey := "etcd_backup_status"
	switch {
	case health.TotalSnapshots == 0:
		health.Error = "No etcd snapshots found"
		alarmCheckDown(alarmKey, fmt.Sprintf("No etcd snapshots found in %s", rke2EtcdSnapshotDir), false, "", "")
	case health.IsLatestTooOld:
		age := time.Since(health.LatestSnapshot.ModTime).Truncate(time.Minute)
		msg := fmt.Sprintf("Latest etcd snapshot is %.1f hours old (max: %d hours) - %s",
			age.Hours(), health.MaxAgeHours, health.LatestSnapshot.Filename)
		alarmCheckDown(alarmKey, msg, false, "", "")
	case health.InvalidSnapshots > 0 && health.ValidSnapshots == 0:
		msg := fmt.Sprintf("All %d etcd snapshots are invalid", health.TotalSnapshots)
		alarmCheckDown(alarmKey, msg, false, "", "")
	case health.InvalidSnapshots > 0:
		msg := fmt.Sprintf("etcd backups: %d valid, %d invalid out of %d total. Latest: %s",
			health.ValidSnapshots, health.InvalidSnapshots, health.TotalSnapshots, health.LatestSnapshot.Filename)
		alarmCheckUp(alarmKey, msg, false)
	default:
		msg := fmt.Sprintf("etcd backups healthy: %d snapshots, latest: %s (%.1f hours ago)",
			health.TotalSnapshots, health.LatestSnapshot.Filename,
			time.Since(health.LatestSnapshot.ModTime).Hours())
		alarmCheckUp(alarmKey, msg, false)
	}

	return health
}

func isCompressedSnapshot(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".zip") || strings.HasSuffix(lower, ".gz") ||
		strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz")
}

const (
	rke2EtcdCACert     = "/var/lib/rancher/rke2/server/tls/etcd/server-ca.crt"
	rke2EtcdClientCert = "/var/lib/rancher/rke2/server/tls/etcd/server-client.crt"
	rke2EtcdClientKey  = "/var/lib/rancher/rke2/server/tls/etcd/server-client.key"
	rke2EtcdEndpoint   = "https://127.0.0.1:2379"
)

type etcdctlEndpointHealth struct {
	Endpoint string `json:"endpoint"`
	Health   bool   `json:"health"`
	Took     string `json:"took"`
}

type etcdctlEndpointStatus struct {
	Endpoint string `json:"Endpoint"`
	Status   struct {
		Header struct {
			ClusterID uint64 `json:"cluster_id"`
			MemberID  uint64 `json:"member_id"`
			Revision  int64  `json:"revision"`
			RaftTerm  uint64 `json:"raft_term"`
		} `json:"header"`
		Version          string `json:"version"`
		DbSize           int64  `json:"dbSize"`
		Leader           uint64 `json:"leader"`
		RaftIndex        uint64 `json:"raftIndex"`
		RaftTerm         uint64 `json:"raftTerm"`
		RaftAppliedIndex uint64 `json:"raftAppliedIndex"`
		DbSizeInUse      int64  `json:"dbSizeInUse"`
	} `json:"Status"`
}

type etcdctlMemberList struct {
	Header struct {
		ClusterID uint64 `json:"cluster_id"`
		MemberID  uint64 `json:"member_id"`
		RaftTerm  uint64 `json:"raft_term"`
	} `json:"header"`
	Members []struct {
		ID         uint64   `json:"ID"`
		Name       string   `json:"name"`
		PeerURLs   []string `json:"peerURLs"`
		ClientURLs []string `json:"clientURLs"`
	} `json:"members"`
}

func etcdctlTLSArgs() []string {
	return []string{
		"--endpoints=" + rke2EtcdEndpoint,
		"--cacert=" + rke2EtcdCACert,
		"--cert=" + rke2EtcdClientCert,
		"--key=" + rke2EtcdClientKey,
	}
}

func hasEtcdTLSCerts() bool {
	for _, path := range []string{rke2EtcdCACert, rke2EtcdClientCert, rke2EtcdClientKey} {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

func runEtcdctl(etcdctlBin string, args ...string) ([]byte, error) {
	fullArgs := append(etcdctlTLSArgs(), args...)
	cmd := exec.Command(etcdctlBin, fullArgs...)
	cmd.Env = append(os.Environ(), "ETCDCTL_API=3")

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return []byte(stdout.String() + stderr.String()), err
	}
	return []byte(stdout.String()), nil
}

func extractJSON(output []byte) string {
	s := strings.TrimSpace(string(output))
	if len(s) == 0 {
		return ""
	}
	if s[0] == '{' || s[0] == '[' {
		return s
	}
	if idx := strings.Index(s, "\n{"); idx >= 0 {
		return strings.TrimSpace(s[idx:])
	}
	if idx := strings.Index(s, "\n["); idx >= 0 {
		return strings.TrimSpace(s[idx:])
	}
	return ""
}

func CollectEtcdClusterStatus() *EtcdClusterStatus {
	status := &EtcdClusterStatus{}

	if !isRKE2Environment() {
		status.Skipped = true
		status.SkipReason = "Not an RKE2 environment"
		return status
	}

	if !hasEtcdTLSCerts() {
		status.Skipped = true
		status.SkipReason = "etcd TLS certificates not found (not a master node)"
		return status
	}

	etcdctlBin := findEtcdctlBinary()
	if etcdctlBin == "" {
		status.Checked = true
		status.Error = "etcdctl binary not found"
		alarmCheckDown("etcd_cluster_health", "etcdctl binary not found, cannot check etcd cluster health", false, "", "")
		return status
	}

	status.Checked = true

	output, err := runEtcdctl(etcdctlBin, "endpoint", "health", "--write-out=json")
	if err != nil {
		status.Error = fmt.Sprintf("etcd health check failed: %v", err)
		status.Healthy = false
		alarmCheckDown("etcd_cluster_health", status.Error, false, "", "")
		return status
	}

	jsonStr := extractJSON(output)
	if jsonStr != "" {
		var healthResults []etcdctlEndpointHealth
		if err := json.Unmarshal([]byte(jsonStr), &healthResults); err == nil && len(healthResults) > 0 {
			status.Healthy = healthResults[0].Health
			status.HealthTook = healthResults[0].Took
		}
	}

	output, err = runEtcdctl(etcdctlBin, "endpoint", "status", "--write-out=json")
	if err == nil {
		jsonStr = extractJSON(output)
		if jsonStr != "" {
			var statusResults []etcdctlEndpointStatus
			if err := json.Unmarshal([]byte(jsonStr), &statusResults); err == nil && len(statusResults) > 0 {
				s := statusResults[0]
				status.Version = s.Status.Version
				status.DbSize = s.Status.DbSize
				status.DbSizeInUse = s.Status.DbSizeInUse
				status.Revision = s.Status.Header.Revision
				status.RaftTerm = s.Status.RaftTerm
				status.RaftIndex = s.Status.RaftIndex
				status.LeaderID = s.Status.Leader
			}
		}
	}

	output, err = runEtcdctl(etcdctlBin, "member", "list", "--write-out=json")
	if err == nil {
		jsonStr = extractJSON(output)
		if jsonStr != "" {
			var memberList etcdctlMemberList
			if err := json.Unmarshal([]byte(jsonStr), &memberList); err == nil {
				status.MemberCount = len(memberList.Members)
				for _, m := range memberList.Members {
					member := EtcdMemberInfo{
						ID:         fmt.Sprintf("%x", m.ID),
						Name:       m.Name,
						IsLeader:   m.ID == status.LeaderID,
						PeerURLs:   m.PeerURLs,
						ClientURLs: m.ClientURLs,
					}
					if member.IsLeader {
						status.LeaderName = m.Name
					}
					status.Members = append(status.Members, member)
				}
			}
		}
	}

	alarmKey := "etcd_cluster_health"
	if status.Healthy {
		msg := fmt.Sprintf("etcd cluster healthy: %d members, leader: %s, version: %s, db: %s",
			status.MemberCount, status.LeaderName, status.Version, formatBytesEtcd(status.DbSize))
		alarmCheckUp(alarmKey, msg, false)
	} else {
		msg := "etcd cluster is NOT healthy"
		if status.Error != "" {
			msg = fmt.Sprintf("etcd cluster is NOT healthy: %s", status.Error)
		}
		alarmCheckDown(alarmKey, msg, false, "", "")
	}

	return status
}

func formatBytesEtcd(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

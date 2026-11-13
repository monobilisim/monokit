//go:build plugin

package k8sHealth

import (
	"context"
	"fmt"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Cluster-wide alarms are the ones built from Kubernetes API data (nodes, pods,
// workloads, cert-manager, compliance, EOL). Every master sees the same API, so
// without coordination each master sends the same alarm under its own name. One
// host is elected through a Lease to send them, labelled with the cluster name.
// Host-local checks (cert files, etcd snapshots, floating IP reachability, RKE2
// config) keep using alarmCheckUp/alarmCheckDown and the host identifier.

const (
	reporterLeaseName           = "monokit-k8shealth-reporter"
	reporterLeaseNamespace      = "kube-system"
	defaultReporterLeaseSeconds = 300
)

// clusterAlarms is decided once per collection run by electClusterAlarmReporter.
// Until then this host reports, so alarms are never lost if the election is skipped.
var clusterAlarms = ClusterAlarmInfo{IsReporter: true, Mode: "single"}

// clusterAlarmName returns the name used to label cluster-wide alarms.
func clusterAlarmName(inCluster bool) string {
	if K8sHealthConfig.K8s.Cluster_name != "" {
		return K8sHealthConfig.K8s.Cluster_name
	}
	identifier := common.Config.Identifier
	if identifier == "" {
		identifier = GetCurrentNodeName()
	}
	// In-cluster there is no node suffix to strip: the identifier names the cluster.
	if inCluster {
		return identifier
	}
	return getClusterNameFromIdentifier(identifier)
}

// reporterIdentity identifies this host in the reporter lease.
func reporterIdentity() string {
	if common.Config.Identifier != "" {
		return common.Config.Identifier
	}
	return GetCurrentNodeName()
}

// electClusterAlarmReporter decides whether this host sends cluster-wide alarms
// during this run and records the outcome in clusterAlarms.
func electClusterAlarmReporter(client kubernetes.Interface, inCluster bool) ClusterAlarmInfo {
	info := ClusterAlarmInfo{
		ClusterName: clusterAlarmName(inCluster),
		Identity:    reporterIdentity(),
		IsReporter:  true,
		Mode:        "single",
	}

	// In-cluster, k8sHealth runs as a single CronJob, so there is nobody to elect against.
	electionEnabled := K8sHealthConfig.K8s.Single_reporter == nil || *K8sHealthConfig.K8s.Single_reporter
	if !electionEnabled || inCluster || client == nil {
		clusterAlarms = info
		return info
	}

	leaseSeconds := defaultReporterLeaseSeconds
	if K8sHealthConfig.K8s.Reporter_lease_seconds != nil && *K8sHealthConfig.K8s.Reporter_lease_seconds > 0 {
		leaseSeconds = *K8sHealthConfig.K8s.Reporter_lease_seconds
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	isReporter, holder, err := acquireReporterLease(ctx, client, info.Identity, leaseSeconds, time.Now())

	info.Mode = "lease"
	info.IsReporter = isReporter
	if holder != info.Identity {
		info.Holder = holder
	}
	if err != nil {
		// Fail open: a duplicate alarm is better than a missed one.
		info.Mode = "fallback"
		info.IsReporter = true
		info.Error = err.Error()
		log.Warn().
			Str("component", "k8sHealth").
			Str("operation", "elect_cluster_alarm_reporter").
			Err(err).
			Msg("Could not use the reporter lease; sending cluster-wide alarms from this host")
	}

	log.Debug().
		Str("component", "k8sHealth").
		Str("operation", "elect_cluster_alarm_reporter").
		Str("cluster", info.ClusterName).
		Str("identity", info.Identity).
		Str("holder", info.Holder).
		Bool("is_reporter", info.IsReporter).
		Str("mode", info.Mode).
		Msg("Cluster alarm reporter decided")

	clusterAlarms = info
	return info
}

// acquireReporterLease takes or renews the reporter lease for identity. It returns
// whether identity holds the lease and who holds it. Another holder keeps the role
// until it stops renewing for leaseSeconds; the holder may renew even after expiry.
// On API errors it returns err, and the caller decides how to fail.
func acquireReporterLease(ctx context.Context, client kubernetes.Interface, identity string, leaseSeconds int, now time.Time) (bool, string, error) {
	leases := client.CoordinationV1().Leases(reporterLeaseNamespace)
	duration := int32(leaseSeconds)
	nowMicro := metav1.NewMicroTime(now)

	lease, err := leases.Get(ctx, reporterLeaseName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		lease = &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{Name: reporterLeaseName, Namespace: reporterLeaseNamespace},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       &identity,
				LeaseDurationSeconds: &duration,
				AcquireTime:          &nowMicro,
				RenewTime:            &nowMicro,
			},
		}
		_, err = leases.Create(ctx, lease, metav1.CreateOptions{})
		if err == nil {
			return true, identity, nil
		}
		if !apierrors.IsAlreadyExists(err) {
			return false, "", fmt.Errorf("creating lease %s/%s: %w", reporterLeaseNamespace, reporterLeaseName, err)
		}
		// Another master created it first; evaluate theirs.
		lease, err = leases.Get(ctx, reporterLeaseName, metav1.GetOptions{})
	}
	if err != nil {
		return false, "", fmt.Errorf("getting lease %s/%s: %w", reporterLeaseNamespace, reporterLeaseName, err)
	}

	holder := ""
	if lease.Spec.HolderIdentity != nil {
		holder = *lease.Spec.HolderIdentity
	}
	if holder != "" && holder != identity && !leaseExpired(lease, now, leaseSeconds) {
		return false, holder, nil
	}

	updated := lease.DeepCopy()
	if holder != identity {
		transitions := int32(0)
		if updated.Spec.LeaseTransitions != nil {
			transitions = *updated.Spec.LeaseTransitions
		}
		transitions++
		updated.Spec.LeaseTransitions = &transitions
		updated.Spec.AcquireTime = &nowMicro
	}
	updated.Spec.HolderIdentity = &identity
	updated.Spec.LeaseDurationSeconds = &duration
	updated.Spec.RenewTime = &nowMicro

	if _, err := leases.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		if apierrors.IsConflict(err) {
			// Someone else wrote the lease since we read it (another master taking
			// over an expired lease). Leave the role to them for this run.
			return false, holder, nil
		}
		return false, holder, fmt.Errorf("updating lease %s/%s: %w", reporterLeaseNamespace, reporterLeaseName, err)
	}
	return true, identity, nil
}

// leaseExpired reports whether the holder has stopped renewing the lease.
func leaseExpired(lease *coordinationv1.Lease, now time.Time, fallbackSeconds int) bool {
	if lease.Spec.RenewTime == nil {
		return true
	}
	seconds := fallbackSeconds
	if lease.Spec.LeaseDurationSeconds != nil && *lease.Spec.LeaseDurationSeconds > 0 {
		seconds = int(*lease.Spec.LeaseDurationSeconds)
	}
	return now.After(lease.Spec.RenewTime.Add(time.Duration(seconds) * time.Second))
}

// clusterAlarmCheckDown raises a cluster-wide alarm. Only the elected reporter sends
// it, labelled with the cluster name. Other hosts drop any state they kept for it
// from when they were the reporter, so it cannot fire stale alarms later.
func clusterAlarmCheckDown(service, message string) {
	if !clusterAlarms.IsReporter {
		common.ClearAlarmState(service)
		return
	}
	alarmCheckDownAs(clusterAlarms.ClusterName, service, message, false, "", "")
}

// clusterAlarmCheckUp is the recovery counterpart of clusterAlarmCheckDown.
func clusterAlarmCheckUp(service, message string) {
	if !clusterAlarms.IsReporter {
		common.ClearAlarmState(service)
		return
	}
	alarmCheckUpAs(clusterAlarms.ClusterName, service, message, false)
}

//go:build plugin

package k8sHealth

import (
	"context"
	"testing"
	"time"

	"github.com/monobilisim/monokit/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAcquireReporterLease(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientset()
	t0 := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

	// First master creates the lease.
	ok, holder, err := acquireReporterLease(ctx, client, "m1", 300, t0)
	if err != nil || !ok || holder != "m1" {
		t.Fatalf("m1 create: ok=%v holder=%q err=%v", ok, holder, err)
	}

	// Other masters back off while m1 keeps it fresh.
	ok, holder, err = acquireReporterLease(ctx, client, "m2", 300, t0.Add(time.Minute))
	if err != nil || ok || holder != "m1" {
		t.Fatalf("m2 while m1 valid: ok=%v holder=%q err=%v", ok, holder, err)
	}
	ok, _, err = acquireReporterLease(ctx, client, "m1", 300, t0.Add(2*time.Minute))
	if err != nil || !ok {
		t.Fatalf("m1 renew: ok=%v err=%v", ok, err)
	}
	ok, _, _ = acquireReporterLease(ctx, client, "m3", 300, t0.Add(6*time.Minute))
	if ok {
		t.Fatalf("m3 must not take a lease renewed 4 minutes ago")
	}

	// m1 stops renewing; after the lease runs out the next master takes over.
	ok, holder, err = acquireReporterLease(ctx, client, "m2", 300, t0.Add(8*time.Minute))
	if err != nil || !ok || holder != "m2" {
		t.Fatalf("m2 takeover: ok=%v holder=%q err=%v", ok, holder, err)
	}
	ok, holder, _ = acquireReporterLease(ctx, client, "m1", 300, t0.Add(9*time.Minute))
	if ok || holder != "m2" {
		t.Fatalf("m1 after takeover: ok=%v holder=%q", ok, holder)
	}

	lease, err := client.CoordinationV1().Leases(reporterLeaseNamespace).Get(ctx, reporterLeaseName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if *lease.Spec.HolderIdentity != "m2" || lease.Spec.LeaseTransitions == nil || *lease.Spec.LeaseTransitions != 1 {
		t.Errorf("unexpected lease spec: holder=%v transitions=%v", *lease.Spec.HolderIdentity, lease.Spec.LeaseTransitions)
	}
}

func TestReporterRenewsOwnExpiredLease(t *testing.T) {
	ctx := context.Background()
	client := fake.NewClientset()
	t0 := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)

	if ok, _, _ := acquireReporterLease(ctx, client, "m1", 300, t0); !ok {
		t.Fatal("m1 should create the lease")
	}
	// The daemon was paused longer than the lease; nobody else claimed it.
	if ok, _, err := acquireReporterLease(ctx, client, "m1", 300, t0.Add(time.Hour)); !ok || err != nil {
		t.Fatalf("m1 should keep its own expired lease: ok=%v err=%v", ok, err)
	}
}

func TestElectClusterAlarmReporter(t *testing.T) {
	prevConfig, prevReporter, prevIdentifier := K8sHealthConfig, clusterAlarms, common.Config.Identifier
	t.Cleanup(func() {
		K8sHealthConfig, clusterAlarms, common.Config.Identifier = prevConfig, prevReporter, prevIdentifier
	})
	K8sHealthConfig = Config{}
	common.Config.Identifier = "mono-ist2-rke2-m2"
	client := fake.NewClientset()

	// m1 already holds a fresh lease.
	if ok, _, err := acquireReporterLease(context.Background(), client, "mono-ist2-rke2-m1", 300, time.Now()); !ok || err != nil {
		t.Fatalf("seed lease: ok=%v err=%v", ok, err)
	}

	info := electClusterAlarmReporter(client, false)
	if info.IsReporter || info.Holder != "mono-ist2-rke2-m1" || info.ClusterName != "mono-ist2-rke2" || info.Mode != "lease" {
		t.Fatalf("unexpected election result: %+v", info)
	}
	if clusterAlarms != info {
		t.Errorf("election result not recorded for the alarm helpers")
	}

	// In-cluster there is a single CronJob: it reports, labelled by its identifier.
	common.Config.Identifier = "k8s-monitor"
	info = electClusterAlarmReporter(client, true)
	if !info.IsReporter || info.ClusterName != "k8s-monitor" || info.Mode != "single" {
		t.Fatalf("unexpected in-cluster result: %+v", info)
	}

	// Election can be turned off, and the cluster name overridden.
	disabled := false
	K8sHealthConfig.K8s.Single_reporter = &disabled
	K8sHealthConfig.K8s.Cluster_name = "ist2-prod"
	common.Config.Identifier = "mono-ist2-rke2-m2"
	info = electClusterAlarmReporter(client, false)
	if !info.IsReporter || info.ClusterName != "ist2-prod" || info.Mode != "single" {
		t.Fatalf("unexpected result with election disabled: %+v", info)
	}
}

//go:build plugin

package k8sHealth

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCollectLegacyMasterTaintCompliance(t *testing.T) {
	legacy := v1.Taint{Key: "node-role.kubernetes.io/master", Effect: v1.TaintEffectNoSchedule}
	modern := v1.Taint{Key: "node-role.kubernetes.io/control-plane", Effect: v1.TaintEffectNoSchedule}

	client := fake.NewClientset(
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "old-master"}, Spec: v1.NodeSpec{Taints: []v1.Taint{legacy, modern}}},
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "new-master"}, Spec: v1.NodeSpec{Taints: []v1.Taint{modern}}},
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}},
	)

	results := CollectLegacyMasterTaintCompliance(client)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	status := make(map[string]bool, len(results))
	for _, r := range results {
		status[r.Resource] = r.Status
	}

	if status["old-master"] {
		t.Error("old-master should fail: deprecated master taint present")
	}
	if !status["new-master"] {
		t.Error("new-master should pass: only control-plane taint")
	}
	if !status["worker"] {
		t.Error("worker should pass: no taints")
	}
}

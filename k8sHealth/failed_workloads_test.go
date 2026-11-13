//go:build plugin

package k8sHealth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/monobilisim/monokit/common"
	"github.com/monobilisim/monokit/common/healthdb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMain(m *testing.M) {
	// Keep the health DB (alarm state) out of the real state directory.
	dir, err := os.MkdirTemp("", "monokit-k8shealth-state")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_STATE_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func ownedPod(namespace, name, ownerKind, ownerName string, labels map[string]string) v1.Pod {
	isController := true
	return v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    labels,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: ownerKind, Name: ownerName, Controller: &isController},
			},
		},
	}
}

func evictedPod(name, rsHash, node, message string) v1.Pod {
	pod := ownedPod("jumpserver-test", name, "ReplicaSet", "kael-ist2-"+rsHash, map[string]string{"pod-template-hash": rsHash})
	pod.Spec.NodeName = node
	pod.Status = v1.PodStatus{Phase: v1.PodFailed, Reason: "Evicted", Message: message}
	return pod
}

func TestFailedPodWorkload(t *testing.T) {
	tests := []struct {
		name     string
		pod      v1.Pod
		wantKind string
		wantName string
		wantOK   bool
	}{
		{
			name:     "deployment replicaset is attributed to the deployment",
			pod:      ownedPod("ns", "app-77fd5b6d7c-78grh", "ReplicaSet", "app-77fd5b6d7c", map[string]string{"pod-template-hash": "77fd5b6d7c"}),
			wantKind: "Deployment", wantName: "app", wantOK: true,
		},
		{
			name:     "replicaset without template hash stays a replicaset",
			pod:      ownedPod("ns", "rs-abcde", "ReplicaSet", "rs", nil),
			wantKind: "ReplicaSet", wantName: "rs", wantOK: true,
		},
		{
			name:     "statefulset",
			pod:      ownedPod("ns", "db-0", "StatefulSet", "db", nil),
			wantKind: "StatefulSet", wantName: "db", wantOK: true,
		},
		{
			name:   "bare pod has no workload",
			pod:    v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "debug"}},
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, name, ok := failedPodWorkload(&tt.pod)
			if ok != tt.wantOK || kind != tt.wantKind || name != tt.wantName {
				t.Fatalf("got (%q, %q, %v), want (%q, %q, %v)", kind, name, ok, tt.wantKind, tt.wantName, tt.wantOK)
			}
		})
	}
}

func TestGroupFailedPodsEvictionLoop(t *testing.T) {
	diskPressure := "The node had condition: [DiskPressure]. "
	pods := []v1.Pod{
		// Two rollouts of the same Deployment evicted on the same node.
		evictedPod("kael-ist2-77fd5b6d7c-78grh", "77fd5b6d7c", "mono-ist2-rke2-w3", diskPressure),
		evictedPod("kael-ist2-77fd5b6d7c-7xxjc", "77fd5b6d7c", "mono-ist2-rke2-w3", diskPressure),
		evictedPod("kael-ist2-68d888d99f-nbq2d", "68d888d99f", "mono-ist2-rke2-w3", diskPressure),
		// Usage figures differ per pod; only the first sentence is kept.
		evictedPod("kael-ist2-68d888d99f-nbztl", "68d888d99f", "mono-ist2-rke2-w2",
			"The node was low on resource: ephemeral-storage. Threshold quantity: 4Gi, available: 3Gi. Container kael was using 2Gi, request is 0."),
	}
	running := ownedPod("jumpserver-test", "kael-ist2-68d888d99f-ok", "ReplicaSet", "kael-ist2-68d888d99f", map[string]string{"pod-template-hash": "68d888d99f"})
	running.Status.Phase = v1.PodRunning
	bare := v1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "jumpserver-test", Name: "debug"}, Status: v1.PodStatus{Phase: v1.PodFailed}}
	pods = append(pods, running, bare)

	nodeConditions := map[string][]string{"mono-ist2-rke2-w3": {"DiskPressure (kubelet has disk pressure)"}}
	groups := groupFailedPods(pods, nodeConditions)

	if len(groups) != 1 {
		t.Fatalf("expected one workload group, got %d: %+v", len(groups), groups)
	}
	g := groups[0]
	if g.Kind != "Deployment" || g.Namespace != "jumpserver-test" || g.Name != "kael-ist2" || g.FailedPods != 4 {
		t.Fatalf("unexpected group: %+v", g)
	}
	wantCauses := []FailureCause{
		{Reason: "Evicted", Message: "The node had condition: [DiskPressure]", Pods: 3},
		{Reason: "Evicted", Message: "The node was low on resource: ephemeral-storage", Pods: 1},
	}
	if len(g.Causes) != len(wantCauses) {
		t.Fatalf("causes = %+v, want %+v", g.Causes, wantCauses)
	}
	for i := range wantCauses {
		if g.Causes[i] != wantCauses[i] {
			t.Errorf("cause %d = %+v, want %+v", i, g.Causes[i], wantCauses[i])
		}
	}
	if len(g.Nodes) != 2 || g.Nodes[0].Name != "mono-ist2-rke2-w3" || g.Nodes[0].Pods != 3 || len(g.Nodes[0].Conditions) != 1 {
		t.Fatalf("unexpected nodes: %+v", g.Nodes)
	}

	msg := g.alarmMessage()
	want := "Deployment 'jumpserver-test/kael-ist2' has 4 failed pod(s). " +
		"Causes: Evicted (3 pods): The node had condition: [DiskPressure]; Evicted (1 pods): The node was low on resource: ephemeral-storage. " +
		"Nodes: mono-ist2-rke2-w3 (3 pods) DiskPressure (kubelet has disk pressure); mono-ist2-rke2-w2 (1 pods)."
	if msg != want {
		t.Errorf("alarm message:\n got: %s\nwant: %s", msg, want)
	}
	if strings.Contains(msg, "78grh") {
		t.Errorf("alarm message must not name individual pods: %s", msg)
	}
}

func TestAlarmMessageSummarizesLongLists(t *testing.T) {
	w := FailedWorkloadInfo{Namespace: "ns", Kind: "Job", Name: "j", FailedPods: 5}
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		w.Causes = append(w.Causes, FailureCause{Reason: "Failed", Message: "m" + n, Pods: 1})
		w.Nodes = append(w.Nodes, FailedWorkloadNode{Name: n, Pods: 1})
	}
	msg := w.alarmMessage()
	if !strings.Contains(msg, "+2 other cause(s)") || !strings.Contains(msg, "+2 other node(s)") {
		t.Errorf("expected overflow summaries, got: %s", msg)
	}
}

func TestPodFailureCauseFallsBackToContainerExit(t *testing.T) {
	pod := v1.Pod{Status: v1.PodStatus{
		Phase: v1.PodFailed,
		ContainerStatuses: []v1.ContainerStatus{{
			Name:  "job",
			State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{Reason: "Error", ExitCode: 2}},
		}},
	}}
	reason, message := podFailureCause(&pod)
	if reason != "Failed" || message != "container job terminated: Error (exit code 2)" {
		t.Errorf("got (%q, %q)", reason, message)
	}
}

func TestNodeProblemConditions(t *testing.T) {
	node := v1.Node{Status: v1.NodeStatus{Conditions: []v1.NodeCondition{
		{Type: v1.NodeMemoryPressure, Status: v1.ConditionFalse, Message: "kubelet has sufficient memory available"},
		{Type: v1.NodeDiskPressure, Status: v1.ConditionTrue, Reason: "KubeletHasDiskPressure", Message: "kubelet has disk pressure"},
		{Type: v1.NodeReady, Status: v1.ConditionTrue, Message: "kubelet is posting ready status"},
	}}}
	got := nodeProblemConditions(&node)
	if len(got) != 1 || got[0] != "DiskPressure (kubelet has disk pressure)" {
		t.Errorf("got %v", got)
	}

	node.Status.Conditions[2] = v1.NodeCondition{Type: v1.NodeReady, Status: v1.ConditionUnknown, Reason: "NodeStatusUnknown"}
	got = nodeProblemConditions(&node)
	if len(got) != 2 || got[1] != "NotReady (NodeStatusUnknown)" {
		t.Errorf("got %v", got)
	}
}

func TestFailedWorkloadAlarmKeyRoundTrip(t *testing.T) {
	key := failedWorkloadAlarmKey("jumpserver-test", "Deployment", "kael-ist2")
	ns, kind, name, ok := parseFailedWorkloadAlarmKey(key)
	if !ok || ns != "jumpserver-test" || kind != "Deployment" || name != "kael-ist2" {
		t.Errorf("round trip of %q gave (%q, %q, %q, %v)", key, ns, kind, name, ok)
	}
	// Must stay clear of the per-pod keys that CleanupOrphanedAlarms deletes.
	for _, suffix := range []string{"_pod_status", "_container_status", "_container"} {
		if strings.Contains(key, suffix) {
			t.Errorf("key %q contains %q", key, suffix)
		}
	}
}

type webhookRecorder struct {
	mu   sync.Mutex
	msgs []string
}

func (r *webhookRecorder) handler(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var body struct{ Text string }
	_ = json.NewDecoder(req.Body).Decode(&body)
	r.msgs = append(r.msgs, body.Text)
	w.WriteHeader(http.StatusOK)
}

func (r *webhookRecorder) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	msgs := r.msgs
	r.msgs = nil
	return msgs
}

func setupClusterAlarmTest(t *testing.T) *webhookRecorder {
	t.Helper()
	rec := &webhookRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(rec.handler))
	t.Cleanup(srv.Close)

	common.ScriptName = "k8sHealth-plugin"
	common.Config.Identifier = "mono-ist2-rke2-m1"
	common.Config.Alarm.Enabled = true
	common.Config.Alarm.Repeat_hours = 0
	common.Config.Alarm.Webhook_urls = []string{srv.URL}
	common.Config.Alarm.SMTP.Enabled = false

	prevConfig, prevReporter := K8sHealthConfig, clusterAlarms
	t.Cleanup(func() { K8sHealthConfig, clusterAlarms = prevConfig, prevReporter })
	enabled, immediate := true, 0.0
	K8sHealthConfig.Alarm.Enabled = &enabled
	K8sHealthConfig.Alarm.Interval = &immediate
	K8sHealthConfig.Alarm.UpInterval = &immediate
	return rec
}

func TestAlarmFailedWorkloadsOncePerWorkloadAsCluster(t *testing.T) {
	rec := setupClusterAlarmTest(t)
	key := failedWorkloadAlarmKey("jumpserver-test", "Deployment", "kael-ist2")
	_ = healthdb.Delete("alarm", key)
	clusterAlarms = ClusterAlarmInfo{ClusterName: "mono-ist2-rke2", IsReporter: true, Mode: "lease"}

	workload := FailedWorkloadInfo{
		Namespace: "jumpserver-test", Kind: "Deployment", Name: "kael-ist2", FailedPods: 13,
		Causes: []FailureCause{{Reason: "Evicted", Message: "The node had condition: [DiskPressure]", Pods: 13}},
	}

	alarmFailedWorkloads([]FailedWorkloadInfo{workload})
	msgs := rec.take()
	if len(msgs) != 1 || !strings.HasPrefix(msgs[0], "[k8sHealth-plugin - mono-ist2-rke2] [:red_circle:] Deployment 'jumpserver-test/kael-ist2' has 13 failed pod(s).") {
		t.Fatalf("expected one cluster-labelled down alarm, got %q", msgs)
	}

	// More pods evicted on the next run: still the same alarm, not a new one.
	workload.FailedPods = 40
	alarmFailedWorkloads([]FailedWorkloadInfo{workload})
	if msgs := rec.take(); len(msgs) != 0 {
		t.Fatalf("expected no repeat while locked, got %q", msgs)
	}

	// Evicted pods cleaned up: one recovery alarm, then the state is gone.
	alarmFailedWorkloads(nil)
	msgs = rec.take()
	if len(msgs) != 1 || msgs[0] != "[k8sHealth-plugin - mono-ist2-rke2] [:check:] Deployment 'jumpserver-test/kael-ist2' has no failed pods any more." {
		t.Fatalf("expected one recovery alarm, got %q", msgs)
	}
	if _, _, _, found, _ := healthdb.GetJSON("alarm", key); found {
		t.Errorf("alarm state should be cleared after recovery")
	}
}

func TestClusterAlarmsSilentOnNonReporter(t *testing.T) {
	rec := setupClusterAlarmTest(t)
	key := failedWorkloadAlarmKey("ns", "Deployment", "app")
	_ = healthdb.Delete("alarm", key)
	workload := FailedWorkloadInfo{Namespace: "ns", Kind: "Deployment", Name: "app", FailedPods: 1}

	// While this host was the reporter it alarmed...
	clusterAlarms = ClusterAlarmInfo{ClusterName: "c", IsReporter: true}
	alarmFailedWorkloads([]FailedWorkloadInfo{workload})
	if msgs := rec.take(); len(msgs) != 1 {
		t.Fatalf("reporter should alarm once, got %q", msgs)
	}

	// ...then another master took over: no alarms, and the stale state is dropped so
	// it cannot send a recovery later.
	clusterAlarms = ClusterAlarmInfo{ClusterName: "c", IsReporter: false, Holder: "c-m2"}
	alarmFailedWorkloads([]FailedWorkloadInfo{workload})
	alarmFailedWorkloads(nil)
	if msgs := rec.take(); len(msgs) != 0 {
		t.Fatalf("non-reporter must not alarm, got %q", msgs)
	}
	if _, _, _, found, _ := healthdb.GetJSON("alarm", key); found {
		t.Errorf("non-reporter should drop its stale alarm state")
	}
}

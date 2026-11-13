//go:build plugin

package k8sHealth

import (
	"fmt"
	"sort"
	"strings"

	"github.com/monobilisim/monokit/common/healthdb"
	"github.com/rs/zerolog/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Failed pods are terminal: their controller replaces them and they stay around until
// deleted. When a node keeps rejecting or evicting pods (e.g. DiskPressure) a
// Deployment can leave hundreds of them behind, each with a new name. Alarming per
// pod name turns that into a flood, so Failed pods with a controller are summarized
// per workload instead, with the eviction message and the node's conditions.

const (
	failedWorkloadAlarmPrefix = "k8s_failed_pods_"
	// How many causes/nodes to name in an alarm before summarizing the rest.
	failedWorkloadAlarmDetailLimit = 3
)

// failedPodWorkload returns the workload a pod belongs to. Pods of a Deployment are
// attributed to the Deployment rather than its ReplicaSet, so pods from successive
// rollouts end up in one group. The Deployment controller names a ReplicaSet
// "<deployment>-<pod-template-hash>", which avoids an API call per ReplicaSet.
func failedPodWorkload(pod *v1.Pod) (kind, name string, ok bool) {
	ref := metav1.GetControllerOf(pod)
	if ref == nil {
		return "", "", false
	}
	if ref.Kind == "ReplicaSet" {
		if hash := pod.Labels["pod-template-hash"]; hash != "" && strings.HasSuffix(ref.Name, "-"+hash) {
			return "Deployment", strings.TrimSuffix(ref.Name, "-"+hash), true
		}
	}
	return ref.Kind, ref.Name, true
}

// isGroupedFailedPod reports whether a pod is summarized per workload instead of
// getting its own pod/container alarms.
func isGroupedFailedPod(pod *v1.Pod) bool {
	if pod.Status.Phase != v1.PodFailed {
		return false
	}
	_, _, ok := failedPodWorkload(pod)
	return ok
}

// failedWorkloadAlarmKey builds the alarm key for a workload. Kubernetes names never
// contain "_", so the parts can be split back out of the key.
func failedWorkloadAlarmKey(namespace, kind, name string) string {
	return failedWorkloadAlarmPrefix + namespace + "_" + kind + "_" + name
}

func parseFailedWorkloadAlarmKey(key string) (namespace, kind, name string, ok bool) {
	if !strings.HasPrefix(key, failedWorkloadAlarmPrefix) {
		return "", "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(key, failedWorkloadAlarmPrefix), "_", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// podFailureCause returns why a Failed pod failed, as kubectl describe shows it.
// Eviction messages are cut to their first sentence: the rest carries per-pod usage
// figures that would otherwise make every pod's message distinct.
func podFailureCause(pod *v1.Pod) (reason, message string) {
	reason = pod.Status.Reason
	if reason == "" {
		reason = "Failed"
	}
	message = strings.TrimSpace(pod.Status.Message)
	if i := strings.Index(message, ". "); i >= 0 {
		message = message[:i]
	}
	message = strings.TrimSuffix(message, ".")
	if message != "" {
		return reason, message
	}
	// No pod-level message (e.g. a Job pod whose container exited non-zero): use the
	// first container that terminated unsuccessfully.
	statuses := append(append([]v1.ContainerStatus{}, pod.Status.InitContainerStatuses...), pod.Status.ContainerStatuses...)
	for _, cs := range statuses {
		if t := cs.State.Terminated; t != nil && t.ExitCode != 0 {
			return reason, fmt.Sprintf("container %s terminated: %s (exit code %d)", cs.Name, t.Reason, t.ExitCode)
		}
	}
	return reason, ""
}

// nodeProblemConditions lists a node's unhealthy conditions, e.g.
// "DiskPressure (kubelet has disk pressure)" or "NotReady (Kubelet stopped posting node status.)".
func nodeProblemConditions(node *v1.Node) []string {
	var problems []string
	for _, c := range node.Status.Conditions {
		var label string
		switch c.Type {
		case v1.NodeReady:
			if c.Status != v1.ConditionTrue {
				label = "NotReady"
			}
		case v1.NodeMemoryPressure, v1.NodeDiskPressure, v1.NodePIDPressure, v1.NodeNetworkUnavailable:
			if c.Status == v1.ConditionTrue {
				label = string(c.Type)
			}
		}
		if label == "" {
			continue
		}
		detail := c.Message
		if detail == "" {
			detail = c.Reason
		}
		if detail != "" {
			label += " (" + detail + ")"
		}
		problems = append(problems, label)
	}
	return problems
}

// groupFailedPods summarizes Failed, controller-owned pods per workload. nodeConditions
// maps node names to their problem conditions. Groups are sorted by namespace, kind, name.
func groupFailedPods(pods []v1.Pod, nodeConditions map[string][]string) []FailedWorkloadInfo {
	type causeKey struct{ reason, message string }
	type group struct {
		info   FailedWorkloadInfo
		causes map[causeKey]int
		nodes  map[string]int
	}
	groups := make(map[string]*group)

	for i := range pods {
		pod := &pods[i]
		if !isGroupedFailedPod(pod) {
			continue
		}
		kind, name, _ := failedPodWorkload(pod)
		key := failedWorkloadAlarmKey(pod.Namespace, kind, name)
		g, ok := groups[key]
		if !ok {
			g = &group{
				info:   FailedWorkloadInfo{Namespace: pod.Namespace, Kind: kind, Name: name},
				causes: make(map[causeKey]int),
				nodes:  make(map[string]int),
			}
			groups[key] = g
		}
		g.info.FailedPods++
		reason, message := podFailureCause(pod)
		g.causes[causeKey{reason, message}]++
		if pod.Spec.NodeName != "" {
			g.nodes[pod.Spec.NodeName]++
		}
	}

	result := make([]FailedWorkloadInfo, 0, len(groups))
	for _, g := range groups {
		for c, n := range g.causes {
			g.info.Causes = append(g.info.Causes, FailureCause{Reason: c.reason, Message: c.message, Pods: n})
		}
		sort.Slice(g.info.Causes, func(i, j int) bool {
			a, b := g.info.Causes[i], g.info.Causes[j]
			if a.Pods != b.Pods {
				return a.Pods > b.Pods
			}
			if a.Reason != b.Reason {
				return a.Reason < b.Reason
			}
			return a.Message < b.Message
		})
		for nodeName, n := range g.nodes {
			g.info.Nodes = append(g.info.Nodes, FailedWorkloadNode{Name: nodeName, Pods: n, Conditions: nodeConditions[nodeName]})
		}
		sort.Slice(g.info.Nodes, func(i, j int) bool {
			a, b := g.info.Nodes[i], g.info.Nodes[j]
			if a.Pods != b.Pods {
				return a.Pods > b.Pods
			}
			return a.Name < b.Name
		})
		result = append(result, g.info)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
	return result
}

// alarmMessage renders the one-line alarm for a failed workload, e.g.
// "Deployment 'ns/app' has 47 failed pod(s). Cause: Evicted (47 pods): The node had
// condition: [DiskPressure]. Node: worker-3 (47 pods) DiskPressure (kubelet has disk pressure)."
func (w FailedWorkloadInfo) alarmMessage() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s '%s/%s' has %d failed pod(s).", w.Kind, w.Namespace, w.Name, w.FailedPods)

	causes := make([]string, 0, len(w.Causes))
	for i, c := range w.Causes {
		if i == failedWorkloadAlarmDetailLimit {
			causes = append(causes, fmt.Sprintf("+%d other cause(s)", len(w.Causes)-i))
			break
		}
		text := fmt.Sprintf("%s (%d pods)", c.Reason, c.Pods)
		if c.Message != "" {
			text += ": " + c.Message
		}
		causes = append(causes, text)
	}
	if len(causes) > 0 {
		label := "Cause"
		if len(w.Causes) > 1 {
			label = "Causes"
		}
		fmt.Fprintf(&sb, " %s: %s.", label, strings.Join(causes, "; "))
	}

	nodes := make([]string, 0, len(w.Nodes))
	for i, n := range w.Nodes {
		if i == failedWorkloadAlarmDetailLimit {
			nodes = append(nodes, fmt.Sprintf("+%d other node(s)", len(w.Nodes)-i))
			break
		}
		text := fmt.Sprintf("%s (%d pods)", n.Name, n.Pods)
		if len(n.Conditions) > 0 {
			text += " " + strings.Join(n.Conditions, ", ")
		}
		nodes = append(nodes, text)
	}
	if len(nodes) > 0 {
		label := "Node"
		if len(w.Nodes) > 1 {
			label = "Nodes"
		}
		fmt.Fprintf(&sb, " %s: %s.", label, strings.Join(nodes, "; "))
	}
	return sb.String()
}

// alarmFailedWorkloads raises one cluster-wide alarm per workload with Failed pods and
// recovers the alarms of workloads whose Failed pods are all gone.
func alarmFailedWorkloads(workloads []FailedWorkloadInfo) {
	current := make(map[string]bool, len(workloads))
	for _, w := range workloads {
		key := failedWorkloadAlarmKey(w.Namespace, w.Kind, w.Name)
		current[key] = true
		clusterAlarmCheckDown(key, w.alarmMessage())
	}

	// Workloads that had an alarm but no Failed pods any more.
	type keyRow struct{ K string }
	var rows []keyRow
	if err := healthdb.Get().Model(&healthdb.KVEntry{}).
		Select("k").
		Where("module = ? AND k LIKE ?", "alarm", failedWorkloadAlarmPrefix+"%").
		Find(&rows).Error; err != nil {
		log.Error().
			Str("component", "k8sHealth").
			Str("operation", "alarm_failed_workloads").
			Err(err).
			Msg("Error querying failed workload alarm keys from health DB")
		return
	}
	for _, r := range rows {
		if current[r.K] {
			continue
		}
		namespace, kind, name, ok := parseFailedWorkloadAlarmKey(r.K)
		if !ok {
			continue
		}
		clusterAlarmCheckUp(r.K, fmt.Sprintf("%s '%s/%s' has no failed pods any more.", kind, namespace, name))
	}
}

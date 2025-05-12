package k8sHealth

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt" // Added fmt for error handling
	"net/http"
	"os"

	// "path/filepath" // Removed as it's unused after recent refactoring
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type CertManager struct {
	APIVersion string `json:"apiVersion"`
	Items      []struct {
		Metadata struct {
			Name string `json:"name"`
		}
		Status struct {
			Conditions []struct {
				LastTransitionTime string `json:"lastTransitionTime"`
				Message            string `json:"message"`
				ObservedGeneration int    `json:"observedGeneration"`
				Reason             string `json:"reason"`
				Status             string `json:"status"`
				Type               string `json:"type"`
			} `json:"conditions"`
			NotAfter    string `json:"notAfter"`
			NotBefore   string `json:"notBefore"`
			RenewalTime string `json:"renewalTime"`
			Revision    int    `json:"revision"`
		} `json:"status"`
	} `json:"items"`
}

var clientset *kubernetes.Clientset

func InitClientset(kubeconfig string) {
	var err error
	// Create a Kubernetes clientset
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		common.LogError("Error creating client config: " + err.Error())
		return
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		common.LogError("Error creating clientset: " + err.Error())
		return
	}
}

// CollectNodeHealth gathers health information for all Kubernetes nodes.
func CollectNodeHealth() ([]NodeHealthInfo, error) {
	common.LogFunctionEntry()
	var nodeHealthInfos []NodeHealthInfo

	if clientset == nil {
		return nil, fmt.Errorf("kubernetes clientset is not initialized")
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		common.LogError("Error listing nodes: " + err.Error())
		return nil, fmt.Errorf("error listing nodes: %w", err)
	}

	for _, node := range nodes.Items {
		role := "worker" // Default role
		// Standard label for master nodes
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
			role = "master"
		}
		// RKE2/K3s and some other distributions use control-plane
		if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
			role = "master"
		}
		// Some older or custom setups might use this
		if val, ok := node.Labels["kubernetes.io/role"]; ok && val == "master" {
			role = "master"
		}

		var status string
		var reason string
		// var message string // Message can be long, omitted from NodeHealthInfo for now
		isReady := false

		for _, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				isReady = condition.Status == v1.ConditionTrue
				status = string(condition.Status) // "True", "False", "Unknown"
				reason = condition.Reason
				// message = condition.Message
				break
			}
		}

		// If NodeReady condition was not found, or to get more details for non-ready nodes
		if status == "" || !isReady {
			for _, condition := range node.Status.Conditions {
				// Prioritize NodeReady if present, even if it's False/Unknown
				if condition.Type == v1.NodeReady {
					status = string(condition.Status)
					reason = condition.Reason
					// message = condition.Message
					isReady = (condition.Status == v1.ConditionTrue)
					break
				}
				// Check for common problematic conditions if NodeReady is missing or True but other issues exist
				if condition.Status == v1.ConditionTrue &&
					(condition.Type == v1.NodeMemoryPressure ||
						condition.Type == v1.NodeDiskPressure ||
						condition.Type == v1.NodePIDPressure ||
						condition.Type == v1.NodeNetworkUnavailable) {
					status = string(condition.Type) // Overwrite status with the problematic condition type
					reason = condition.Reason
					// message = condition.Message
					isReady = false // Explicitly false if these are true
					break
				}
			}
		}

		// If still no specific status (e.g. NodeReady condition was entirely missing)
		if status == "" {
			status = "Unknown"
			reason = "NoNodeConditionsFound"
			// message = "No conditions reported for the node."
		}

		nodeHealthInfos = append(nodeHealthInfos, NodeHealthInfo{
			Name:    node.Name,
			Role:    role,
			Status:  status,
			Reason:  reason,
			IsReady: isReady,
		})
	}
	return nodeHealthInfos, nil
}

// CollectPodRunningLogChecks was removed as per user request.
// func CollectPodRunningLogChecks() ([]PodLogCheckInfo, error) { ... }

// CollectRke2IngressNginxHealth checks the RKE2 Ingress Nginx configuration and related floating IPs.
func CollectRke2IngressNginxHealth() (*Rke2IngressNginxHealth, error) {
	common.LogFunctionEntry()
	health := &Rke2IngressNginxHealth{
		FloatingIPChecks: make([]FloatingIPCheck, 0),
	}

	ingressNginxYamlPath := "/var/lib/rancher/rke2/server/manifests/rke2-ingress-nginx.yaml"
	if !common.FileExists(ingressNginxYamlPath) {
		ingressNginxYamlPath = "/var/lib/rancher/rke2/server/manifests/rke2-ingress-nginx-config.yaml"
	}
	health.ManifestPath = ingressNginxYamlPath

	if common.FileExists(ingressNginxYamlPath) {
		health.ManifestAvailable = true
		common.AlarmCheckUp("rke2_ingress_nginx_manifest", fmt.Sprintf("RKE2 Ingress Nginx manifest found: %s", ingressNginxYamlPath), false)

		// Use a new Viper instance to avoid global state issues if this func is called multiple times
		// or if other parts of monokit use viper globally for different configs.
		v := viper.New()
		v.SetConfigFile(ingressNginxYamlPath)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			errMsg := fmt.Sprintf("Error reading RKE2 Ingress Nginx manifest %s: %v", ingressNginxYamlPath, err)
			common.LogError(errMsg)
			health.Error = errMsg
			// No alarm here, as the manifest exists but is unreadable. UI will show error.
			// Or, common.AlarmCheckDown("rke2_ingress_nginx_config_read", errMsg, false, "", "")
		} else {
			// Check for spec.valuesContent.controller.service.enabled
			// Using IsSet to differentiate between 'false' and 'not present'
			if v.IsSet("spec.valuesContent.controller.service.enabled") {
				val := v.GetBool("spec.valuesContent.controller.service.enabled")
				health.PublishServiceEnabled = &val // Store pointer to bool
				if val {
					common.AlarmCheckUp("rke2_ingress_nginx_publishservice", "RKE2 Ingress Nginx: PublishService is enabled.", false)
				} else {
					common.AlarmCheckDown("rke2_ingress_nginx_publishservice", "RKE2 Ingress Nginx: PublishService is NOT enabled in manifest.", false, "", "")
				}
			} else {
				common.LogDebug(fmt.Sprintf("RKE2 Ingress Nginx: spec.valuesContent.controller.service.enabled not set in %s", ingressNginxYamlPath))
				// health.PublishServiceEnabled remains nil
			}

			// The original code checked the same path twice for "publishService" and "service".
			// Assuming "service" was a typo and it meant to check the same path.
			// If "service" is a different path, this needs adjustment.
			// For now, I'll assume it's the same as PublishServiceEnabled.
			// If there's a distinct "service.enabled" path, it should be:
			// if v.IsSet("spec.another.path.service.enabled") { ... health.ServiceEnabled = &val ... }
			// For now, let's assume the original code meant the same key or it's redundant.
			// To match original logic's variable names:
			if v.IsSet("spec.valuesContent.controller.service.enabled") { // Assuming this is what 'service' referred to
				val := v.GetBool("spec.valuesContent.controller.service.enabled")
				health.ServiceEnabled = &val
				// Alarm for this specific key if needed, or rely on PublishServiceEnabled alarm.
			}
		}
	} else {
		health.ManifestAvailable = false
		errMsg := fmt.Sprintf("RKE2 Ingress Nginx manifest not found at %s or alternate path.", ingressNginxYamlPath)
		common.LogWarn(errMsg) // It's a warning as it might not be an RKE2 setup
		common.AlarmCheckDown("rke2_ingress_nginx_manifest", errMsg, false, "", "")
		// health.Error = errMsg // Not necessarily an error for the overall k8s health if RKE2 ingress is not expected.
		// UI will show manifest not available.
	}

	// Test Ingress Floating IPs
	if len(K8sHealthConfig.K8s.Ingress_Floating_Ips) > 0 {
		for _, floatingIp := range K8sHealthConfig.K8s.Ingress_Floating_Ips {
			check := FloatingIPCheck{IP: floatingIp, TestType: "ingress"}
			// Equivalent of `curl -o /dev/null -s -w "%{http_code}\n" http://$floatingIp`
			// Using a timeout for the HTTP client
			client := http.Client{Timeout: 5 * time.Second}
			resp, err := client.Get("http://" + floatingIp)

			if err != nil {
				check.IsAvailable = false
				check.StatusCode = 0 // Or some indicator of connection error
				common.LogError(fmt.Sprintf("Error checking ingress floating IP %s: %v", floatingIp, err))
				common.AlarmCheckDown("floating_ip_ingress_"+floatingIp, fmt.Sprintf("Ingress Floating IP %s is not reachable: %v", floatingIp, err), false, "", "")
			} else {
				defer resp.Body.Close()
				check.StatusCode = resp.StatusCode
				// For ingress, 404 is often the default backend's "OK" response if no specific ingress matches.
				// Other codes like 200, 3xx might also be acceptable depending on setup.
				// The original check considered only 404 as "true".
				if resp.StatusCode == http.StatusNotFound { // 404
					check.IsAvailable = true
					common.AlarmCheckUp("floating_ip_ingress_"+floatingIp, fmt.Sprintf("Ingress Floating IP %s is available (HTTP %d).", floatingIp, resp.StatusCode), false)
				} else {
					check.IsAvailable = false // Or true, if other codes are acceptable. For now, matching original.
					common.AlarmCheckDown("floating_ip_ingress_"+floatingIp, fmt.Sprintf("Ingress Floating IP %s returned HTTP %d (expected 404 or other success).", floatingIp, resp.StatusCode), false, "", "")
				}
			}
			health.FloatingIPChecks = append(health.FloatingIPChecks, check)
		}
	} else {
		common.LogDebug("No Ingress Floating IPs configured to check.")
	}

	return health, nil // No top-level error from this function itself unless fundamental issue
}

// CollectPodHealth gathers health information for all pods in all namespaces.
func CollectPodHealth() ([]PodHealthInfo, error) {
	common.LogFunctionEntry()
	var podInfos []PodHealthInfo

	if clientset == nil {
		return nil, fmt.Errorf("kubernetes clientset is not initialized")
	}

	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		common.LogError("Error listing pods: " + err.Error())
		return nil, fmt.Errorf("error listing pods: %w", err)
	}

	for _, pod := range pods.Items {
		podInfo := PodHealthInfo{
			Namespace:       pod.Namespace,
			Name:            pod.Name,
			Phase:           string(pod.Status.Phase),
			ContainerStates: make([]ContainerHealthInfo, 0),
		}

		isPodProblemByPhase := false
		podOverallHealthy := true // Assume healthy until a problem is found

		// Determine pod problem status based on phase
		switch pod.Status.Phase {
		case v1.PodFailed, v1.PodUnknown:
			isPodProblemByPhase = true
			podOverallHealthy = false
		case v1.PodPending:
			// Pending can be normal, but if containers are failing to start, it's a problem.
			// This will be further evaluated by container states.
			// For now, consider Pending as potentially problematic for alarm purposes if it persists or has bad containers.
			isPodProblemByPhase = true // Tentatively problematic; container checks will confirm
		case v1.PodRunning:
			// Healthy by phase, but individual containers might have issues.
			isPodProblemByPhase = false
		case v1.PodSucceeded:
			// Completed successfully, not a problem.
			isPodProblemByPhase = false
		default:
			isPodProblemByPhase = true // Any other phase is unexpected/problematic
			podOverallHealthy = false
		}

		// Check container statuses
		allContainersHealthy := true
		for _, cs := range pod.Status.ContainerStatuses {
			containerInfo := ContainerHealthInfo{
				Name: cs.Name,
			}
			if cs.State.Running != nil {
				containerInfo.State = "Running"
				containerInfo.IsReady = true // Simplified: Running means ready for basic checks
			} else if cs.State.Waiting != nil {
				containerInfo.State = "Waiting"
				containerInfo.Reason = cs.State.Waiting.Reason
				containerInfo.Message = cs.State.Waiting.Message
				containerInfo.IsReady = false
				allContainersHealthy = false
			} else if cs.State.Terminated != nil {
				containerInfo.State = "Terminated"
				containerInfo.Reason = cs.State.Terminated.Reason
				containerInfo.Message = cs.State.Terminated.Message
				// A container that terminated with "Completed" is fine.
				if cs.State.Terminated.Reason == "Completed" {
					containerInfo.IsReady = true // Or a different status like "Completed"
				} else {
					containerInfo.IsReady = false
					allContainersHealthy = false
				}
			} else {
				containerInfo.State = "Unknown"
				containerInfo.IsReady = false
				allContainersHealthy = false
			}
			podInfo.ContainerStates = append(podInfo.ContainerStates, containerInfo)

			// Container-level alarms
			containerAlarmKey := fmt.Sprintf("%s-%s-%s_container_status", pod.Namespace, pod.Name, cs.Name)
			if !containerInfo.IsReady && containerInfo.State != "Terminated" && containerInfo.Reason != "Completed" { // Avoid alarming for completed containers
				alarmMsg := fmt.Sprintf("Container '%s' in pod '%s/%s' is in state %s (Reason: %s, Message: %s)",
					cs.Name, pod.Namespace, pod.Name, containerInfo.State, containerInfo.Reason, containerInfo.Message)
				common.AlarmCheckDown(containerAlarmKey, alarmMsg, false, "", "")
			} else {
				alarmMsg := fmt.Sprintf("Container '%s' in pod '%s/%s' is healthy (State: %s).",
					cs.Name, pod.Namespace, pod.Name, containerInfo.State)
				common.AlarmCheckUp(containerAlarmKey, alarmMsg, false)
			}
		}

		// Check init container statuses as well, if any
		for _, ics := range pod.Status.InitContainerStatuses {
			initContainerInfo := ContainerHealthInfo{
				Name: ics.Name + " (init)", // Mark as init container
			}
			if ics.State.Running != nil {
				initContainerInfo.State = "Running"
				initContainerInfo.IsReady = false // Init containers are not "Ready" in the same way, but are not done.
				allContainersHealthy = false      // Pod is not fully up if init containers are running
			} else if ics.State.Waiting != nil {
				initContainerInfo.State = "Waiting"
				initContainerInfo.Reason = ics.State.Waiting.Reason
				initContainerInfo.Message = ics.State.Waiting.Message
				initContainerInfo.IsReady = false
				allContainersHealthy = false
			} else if ics.State.Terminated != nil {
				initContainerInfo.State = "Terminated"
				initContainerInfo.Reason = ics.State.Terminated.Reason
				initContainerInfo.Message = ics.State.Terminated.Message
				if ics.State.Terminated.Reason == "Completed" {
					initContainerInfo.IsReady = true // Successfully completed
				} else {
					initContainerInfo.IsReady = false // Failed
					allContainersHealthy = false
				}
			} else {
				initContainerInfo.State = "Unknown"
				initContainerInfo.IsReady = false
				allContainersHealthy = false
			}
			podInfo.ContainerStates = append(podInfo.ContainerStates, initContainerInfo)

			// Init Container-level alarms
			initContainerAlarmKey := fmt.Sprintf("%s-%s-%s_init_container_status", pod.Namespace, pod.Name, ics.Name)
			if !initContainerInfo.IsReady && initContainerInfo.State != "Terminated" && initContainerInfo.Reason != "Completed" {
				alarmMsg := fmt.Sprintf("Init container '%s' in pod '%s/%s' is in state %s (Reason: %s, Message: %s)",
					ics.Name, pod.Namespace, pod.Name, initContainerInfo.State, initContainerInfo.Reason, initContainerInfo.Message)
				common.AlarmCheckDown(initContainerAlarmKey, alarmMsg, false, "", "")
			} else {
				alarmMsg := fmt.Sprintf("Init container '%s' in pod '%s/%s' is healthy or completed (State: %s).",
					ics.Name, pod.Namespace, pod.Name, initContainerInfo.State)
				common.AlarmCheckUp(initContainerAlarmKey, alarmMsg, false)
			}
		}

		if !allContainersHealthy && pod.Status.Phase == v1.PodRunning {
			podOverallHealthy = false // If pod is running but containers are not, it's a problem
		}
		if isPodProblemByPhase && pod.Status.Phase == v1.PodPending && allContainersHealthy {
			// If pending but all containers look okay (e.g. waiting for resources, not error states)
			// then it might not be an immediate "problem" for IsProblem flag, but still "Pending".
			// The UI can show "Pending". For alarms, "Pending" is often a state to watch.
			// The original code treated Pending as not an issue for pod-level alarm unless it was NOT Pending/Running/Succeeded.
			// Let's stick to: if phase is Pending, and no container is in a definitive error state, podOverallHealthy remains true for now.
			// The alarm logic below will handle "Pending" specifically.
			podOverallHealthy = true // Re-evaluate: Pending is not Running/Succeeded.
		}

		podInfo.IsProblem = !podOverallHealthy || isPodProblemByPhase
		// Refine IsProblem: A pod is a problem if its phase is Failed/Unknown,
		// OR if its phase is Running/Pending but not all containers are ready/completed.
		if pod.Status.Phase == v1.PodSucceeded {
			podInfo.IsProblem = false
		} else if pod.Status.Phase == v1.PodRunning && allContainersHealthy {
			podInfo.IsProblem = false
		} else {
			podInfo.IsProblem = true
		}

		// Pod-level alarms
		podAlarmKey := fmt.Sprintf("%s-%s_pod_status", pod.Namespace, pod.Name)
		if podInfo.IsProblem {
			// More detailed message for problematic pods
			var problemDetails []string
			if pod.Status.Phase != v1.PodRunning && pod.Status.Phase != v1.PodSucceeded {
				problemDetails = append(problemDetails, fmt.Sprintf("phase is %s", pod.Status.Phase))
			}
			for _, cs := range podInfo.ContainerStates {
				if !cs.IsReady && !(cs.State == "Terminated" && cs.Reason == "Completed") {
					problemDetails = append(problemDetails, fmt.Sprintf("container %s is %s (Reason: %s)", cs.Name, cs.State, cs.Reason))
				}
			}
			alarmMsg := fmt.Sprintf("Pod '%s/%s' is problematic: %s.",
				pod.Namespace, pod.Name, strings.Join(problemDetails, "; "))
			common.AlarmCheckDown(podAlarmKey, alarmMsg, false, "", "")
		} else {
			alarmMsg := fmt.Sprintf("Pod '%s/%s' is healthy (Phase: %s).", pod.Namespace, pod.Name, pod.Status.Phase)
			common.AlarmCheckUp(podAlarmKey, alarmMsg, false)
		}
		podInfos = append(podInfos, podInfo)
	}
	return podInfos, nil
}

// CollectCertManagerHealth checks the status of cert-manager and its certificates.
func CollectCertManagerHealth() (*CertManagerHealth, error) {
	common.LogFunctionEntry()
	health := &CertManagerHealth{
		Certificates: make([]CertificateInfo, 0),
	}

	if clientset == nil {
		health.Error = "kubernetes clientset is not initialized"
		return health, fmt.Errorf(health.Error)
	}

	// Check cert-manager namespace
	_, err := clientset.CoreV1().Namespaces().Get(context.TODO(), "cert-manager", metav1.GetOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			health.NamespaceAvailable = false
			health.Error = "cert-manager namespace not found. Cert-manager might not be installed."
			//common.LogWarn(health.Error)
			// This is not necessarily a critical error for the whole k8s check,
			// but cert-manager specific checks cannot proceed.
			common.AlarmCheckDown("cert_manager_namespace", health.Error, false, "", "")
			return health, nil // Return current health, not a fatal error for the collector.
		}
		errMsg := fmt.Sprintf("Error getting cert-manager namespace: %v", err)
		common.LogError(errMsg)
		health.Error = errMsg
		common.AlarmCheckDown("cert_manager_namespace", errMsg, false, "", "")
		return health, fmt.Errorf(errMsg) // This is a more significant k8s API error.
	}
	health.NamespaceAvailable = true
	common.AlarmCheckUp("cert_manager_namespace", "cert-manager namespace exists.", false)

	// Get a list of cert-manager.io/Certificate resources
	rawCertData, err := clientset.RESTClient().Get().AbsPath("/apis/cert-manager.io/v1/certificates").DoRaw(context.Background())
	if err != nil {
		errMsg := fmt.Sprintf("Error getting cert-manager.io/Certificate resources: %v", err)
		common.LogError(errMsg)
		health.Error = errMsg
		// If CRDs are not installed, this will fail.
		if strings.Contains(err.Error(), "the server could not find the requested resource") {
			common.LogWarn("Cert-manager CRDs for Certificates might not be installed.")
			common.AlarmCheckDown("cert_manager_crd_certificates", "Cert-manager Certificate CRD not found.", false, "", "")
		} else {
			common.AlarmCheckDown("cert_manager_api_certificates", errMsg, false, "", "")
		}
		return health, nil // Not a fatal error for the collector if CRDs are missing.
	}

	var certManagerCR CertManager // Using the existing CertManager struct for parsing
	if err := json.Unmarshal(rawCertData, &certManagerCR); err != nil {
		errMsg := fmt.Sprintf("Error parsing cert-manager Certificate JSON: %v", err)
		common.LogError(errMsg)
		health.Error = errMsg
		common.AlarmCheckDown("cert_manager_json_parse", errMsg, false, "", "")
		return health, fmt.Errorf(errMsg) // Parsing error is more critical
	}

	for _, item := range certManagerCR.Items {
		certInfo := CertificateInfo{
			Name: item.Metadata.Name,
			// NotAfter, NotBefore, RenewalTime are not in CertificateInfo struct in types.go
			// These were likely from an older version or a direct mapping of the CRD status.
			// The CertificateInfo struct in types.go focuses on IsReady, Reason, Message.
			// If these fields are needed for display, CertificateInfo in types.go needs to be updated.
			// For now, removing them to match the defined struct.
			// NotAfter:    item.Status.NotAfter,
			// NotBefore:   item.Status.NotBefore,
			// RenewalTime: item.Status.RenewalTime,
		}

		isReady := false
		var readyConditionMessage string
		for _, condition := range item.Status.Conditions {
			if condition.Type == "Ready" {
				isReady = condition.Status == "True"
				readyConditionMessage = condition.Message
				break
			}
		}
		certInfo.IsReady = isReady
		certInfo.Message = readyConditionMessage

		alarmKey := fmt.Sprintf("cert_manager_cert_%s_ready", item.Metadata.Name)
		if !isReady {
			alarmMsg := fmt.Sprintf("Certificate '%s' is not Ready. Message: %s", item.Metadata.Name, readyConditionMessage)
			common.AlarmCheckDown(alarmKey, alarmMsg, false, "", "")
		} else {
			alarmMsg := fmt.Sprintf("Certificate '%s' is Ready.", item.Metadata.Name)
			common.AlarmCheckUp(alarmKey, alarmMsg, false)
		}
		health.Certificates = append(health.Certificates, certInfo)
	}

	if len(health.Certificates) == 0 && health.Error == "" {
		common.LogInfo("No cert-manager certificates found.")
	}

	return health, nil
}

// CollectKubeVipHealth gathers Kube-VIP status and floating IP reachability.
func CollectKubeVipHealth() (*KubeVipHealth, error) {
	common.LogFunctionEntry()
	health := &KubeVipHealth{
		FloatingIPChecks: make([]FloatingIPCheck, 0),
	}

	if clientset == nil {
		health.Error = "kubernetes clientset is not initialized"
		return health, fmt.Errorf(health.Error)
	}

	// Check if kube-vip pods exists on kube-system namespace
	pods, err := clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		errMsg := fmt.Sprintf("Error listing pods in kube-system for Kube-VIP check: %v", err)
		common.LogError(errMsg)
		health.Error = errMsg
		// Do not return yet, as Kube-VIP might not be installed, which is not a fatal error for the collector.
	} else {
		for _, pod := range pods.Items {
			if strings.Contains(pod.Name, "kube-vip") {
				health.PodsAvailable = true
				break
			}
		}
	}

	if health.PodsAvailable {
		common.AlarmCheckUp("kube_vip_pods", "Kube-VIP pods detected in kube-system.", false)
		if len(K8sHealthConfig.K8s.Floating_Ips) > 0 {
			for _, floatingIp := range K8sHealthConfig.K8s.Floating_Ips {
				check := FloatingIPCheck{IP: floatingIp, TestType: "kube-vip"}
				pinger, err := probing.NewPinger(floatingIp)
				if err != nil {
					common.LogError(fmt.Sprintf("Error creating pinger for Kube-VIP IP %s: %v", floatingIp, err))
					check.IsAvailable = false
					// Optionally add a message to the check or health.Error
				} else {
					pinger.Count = 1
					pinger.Timeout = 3 * time.Second // Reduced timeout for quicker checks
					err = pinger.Run()
					if err != nil {
						check.IsAvailable = false
						common.AlarmCheckDown("floating_ip_kube_vip_"+floatingIp, fmt.Sprintf("Kube-VIP Floating IP %s is not reachable: %v", floatingIp, err), false, "", "")
					} else {
						check.IsAvailable = true
						common.AlarmCheckUp("floating_ip_kube_vip_"+floatingIp, fmt.Sprintf("Kube-VIP Floating IP %s is reachable.", floatingIp), false)
					}
				}
				health.FloatingIPChecks = append(health.FloatingIPChecks, check)
			}
		} else {
			common.LogDebug("No Kube-VIP Floating IPs configured to check.")
		}
	} else {
		common.LogDebug("Kube-VIP pods not detected in kube-system.")
		common.AlarmCheckDown("kube_vip_pods", "Kube-VIP pods not detected in kube-system. This might be normal if Kube-VIP is not used.", false, "", "")
	}
	return health, nil // Return health, error primarily for client init or major issues
}

// CollectClusterApiCertHealth checks the RKE2 API server certificate.
func CollectClusterApiCertHealth() (*ClusterApiCertHealth, error) {
	common.LogFunctionEntry()
	health := &ClusterApiCertHealth{}
	crtFile := "/var/lib/rancher/rke2/server/tls/serving-kube-apiserver.crt"
	health.CertFilePath = crtFile

	if !common.FileExists(crtFile) {
		errMsg := fmt.Sprintf("Cluster API server certificate file not found: %s", crtFile)
		common.LogWarn(errMsg) // This might be normal if not an RKE2 master
		health.Error = errMsg
		health.CertFileAvailable = false
		common.AlarmCheckDown("kube_apiserver_cert_file", errMsg, false, "", "")
		return health, nil // Not a fatal error for the collector
	}
	health.CertFileAvailable = true
	common.AlarmCheckUp("kube_apiserver_cert_file", fmt.Sprintf("Cluster API server certificate file found: %s", crtFile), false)

	certFileContent, err := os.ReadFile(crtFile)
	if err != nil {
		errMsg := fmt.Sprintf("Error reading Cluster API server certificate file %s: %v", crtFile, err)
		common.LogError(errMsg)
		health.Error = errMsg
		common.AlarmCheckDown("kube_apiserver_cert_read", errMsg, false, "", "")
		return health, fmt.Errorf(errMsg) // This is a file read error
	}

	block, _ := pem.Decode(certFileContent)
	if block == nil {
		errMsg := fmt.Sprintf("Failed to parse PEM block from Cluster API server certificate file: %s", crtFile)
		common.LogError(errMsg)
		health.Error = errMsg
		common.AlarmCheckDown("kube_apiserver_cert_parse", errMsg, false, "", "")
		return health, fmt.Errorf(errMsg)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		errMsg := fmt.Sprintf("Error parsing Cluster API server certificate from %s: %v", crtFile, err)
		common.LogError(errMsg)
		health.Error = errMsg
		common.AlarmCheckDown("kube_apiserver_cert_parse", errMsg, false, "", "")
		return health, fmt.Errorf(errMsg)
	}

	health.NotAfter = cert.NotAfter
	health.IsExpired = cert.NotAfter.Before(time.Now())

	if health.IsExpired {
		alarmMsg := fmt.Sprintf("Cluster API server certificate (%s) is EXPIRED. Expires: %s", crtFile, health.NotAfter.Format(time.RFC3339))
		common.AlarmCheckDown("kube_apiserver_cert_expiry", alarmMsg, false, "", "")
	} else {
		alarmMsg := fmt.Sprintf("Cluster API server certificate (%s) is valid. Expires: %s", crtFile, health.NotAfter.Format(time.RFC3339))
		common.AlarmCheckUp("kube_apiserver_cert_expiry", alarmMsg, false)
	}

	return health, nil
}

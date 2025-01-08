package k8sHealth

import (
    "os"
    "time"
    "strings"
    "strconv"
    "context"
    "net/http"
    "crypto/x509"
    "encoding/pem"
    "path/filepath"
    "encoding/json"
    v1 "k8s.io/api/core/v1"
    "github.com/spf13/viper"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
    "github.com/monobilisim/monokit/common"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    probing "github.com/prometheus-community/pro-bing"
)

type CertManager struct {
	APIVersion string `json:"apiVersion"`
	Items      []struct {
        Metadata    struct {
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

func CheckNodes(master bool) {
    // Get all the nodes
    nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        common.LogError(err.Error())
    }

    // Iterate over all the nodes
    for _, node := range nodes.Items {
        _, ok := node.Labels["node-role.kubernetes.io/master"]
        if ok == master {
            var isReady bool
            for _, condition := range node.Status.Conditions {
                if condition.Type == v1.NodeReady {
                    isReady = condition.Status == v1.ConditionTrue
                }
            }

            if isReady == false {
                common.AlarmCheckDown(string(node.Name) + "_ready", "Node " + node.Name + " is not Ready, is in " + string(node.Status.Conditions[0].Type), false)
                common.PrettyPrintStr(string(node.Name), false, "Ready")
            } else {
                common.AlarmCheckUp(string(node.Name) + "_ready", "Node " + node.Name + " is now Ready", false)
                common.PrettyPrintStr(string(node.Name), true, "Ready")
            }
        }
    }
}

func CheckPodRunningLogs() {
    // Get all under TmpDir
    files, err := os.ReadDir(common.TmpDir)
    if err != nil {
        common.LogError(err.Error())
    }

    var podExists bool
    
    // List all pods
    pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        common.LogError(err.Error())
    }

    // Iterate over all the files
    for _, file := range files {
        if strings.Contains(file.Name(), "_running.log") {
            // Is a pod, so we split the pod name out of it
            podName := strings.Split(file.Name(), "_running.log")[0]

            // Iterate over all the pods
            for _, pod := range pods.Items {
                if pod.Name == podName {
                    podExists = true
                    break
                }
            }
            
            if podExists == false {
                common.AlarmCheckUp(podName + "_running", "Pod '" + podName + "' doesn't exist anymore, likely replaced", false)
            }
        }
    }
}
                

func CheckRke2IngressNginx() {
    ingressNginxYaml := "/var/lib/rancher/rke2/server/manifests/rke2-ingress-nginx.yaml"

    // Check if the file exists
    if common.FileExists(ingressNginxYaml) == false {
        ingressNginxYaml = "/var/lib/rancher/rke2/server/manifests/rke2-ingress-nginx-config.yaml"
    }

    if common.FileExists(ingressNginxYaml) {
        // Get filename from ingressNginxYaml
        common.PrettyPrintStr(filepath.Base(ingressNginxYaml), true, "available")
        viper.SetConfigFile(ingressNginxYaml)
        viper.SetConfigType("yaml")
        err := viper.ReadInConfig()
        if err != nil {
            common.LogError(err.Error())
        }
        publishService := viper.GetBool("spec.valuesContent.controller.service.enabled")
        service := viper.GetBool("spec.valuesContent.controller.service.enabled")

        common.PrettyPrintStr("publishService", publishService, "enabled")

        common.PrettyPrintStr("service", service, "enabled")
    } else {
        common.PrettyPrintStr(filepath.Base(ingressNginxYaml), false, "available")
    }

    // Test floating IPs
    for _, floatingIp := range K8sHealthConfig.K8s.Ingress_Floating_Ips {
        // equivilant of `curl -o /dev/null -s -w "%{http_code}\n" http://$floatingIp`
        response, _ := http.Get("http://" + floatingIp)
            
        // Get response code
        if response.StatusCode == 404 {
            common.PrettyPrintStr(floatingIp, true, "available and returned HTTP " + strconv.Itoa(response.StatusCode))
        } else {
            common.PrettyPrintStr(floatingIp, false, "available or returned HTTP " + strconv.Itoa(response.StatusCode))
            common.AlarmCheckDown("floating_unexpected_ " + floatingIp, "Floating IP " + floatingIp + " is not available or returned HTTP " + strconv.Itoa(response.StatusCode), false)
        }
    }
}

func podAlarmCheckDown(podName string, namespace string, actualStatus string) {
    
    podStillExists := false

    pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        common.LogError(err.Error())
    }

    for _, pod := range pods.Items {
        if pod.Name == podName {
            podStillExists = true
        }
    }

    if podStillExists == false {
       common.AlarmCheckUp(podName + "_running", "Pod '" + podName + "' from namespace '" + namespace + "' doesn't exist anymore, likely replaced", false)
    } else {
        common.AlarmCheckDown(podName + "_running", "Pod " + podName + " is " + actualStatus, false)
    }
}

func CheckPods() { 
    firstTime := true

    // Get all the pods
    pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        common.LogError(err.Error())
    }
    
    // Iterate over all the pods
    for _, pod := range pods.Items {

            
        if pod.Status.Phase != v1.PodRunning && pod.Status.Phase != v1.PodSucceeded && pod.Status.Phase != v1.PodPending {
            if firstTime {
                common.SplitSection("Pods:")
                firstTime = false
            }
            podAlarmCheckDown(string(pod.Name), string(pod.Namespace), string(pod.Status.Phase))
            common.PrettyPrintStr(string(pod.Name), false, "Running")
        } else {
            common.AlarmCheckUp(string(pod.Name) + "_running", "Pod " + pod.Name + " is now " + string(pod.Status.Phase), false)
        }


        for _, containerStatus := range pod.Status.ContainerStatuses {
            if containerStatus.State.Running != nil && containerStatus.State.Waiting == nil && containerStatus.State.Terminated == nil {
                common.AlarmCheckUp(pod.Name + "_container", "Container '" + containerStatus.Name + "' from pod '" + pod.Name + "' is now Running", false)
                continue
            }

            if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.Reason == "Completed" {
                continue
            }
            
            if firstTime {
                common.SplitSection("Pods:")
                firstTime = false
            }
            

            if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.Reason != "Completed" {
                common.PrettyPrintStr("Container " + containerStatus.Name + " from pod " + pod.Name, false, "Running")
               common.AlarmCheckDown(pod.Name + "_container", "Container '" + containerStatus.Name + "' from pod '" + pod.Name + "' is terminated with Reason '" + containerStatus.State.Terminated.Reason + "'", false)
            }

            if containerStatus.State.Waiting != nil {
                common.PrettyPrintStr("Container " + containerStatus.Name + " from pod " + pod.Name, false, "Running")
                common.AlarmCheckDown(pod.Name + "_container", "Container '" + containerStatus.Name + "' from pod '" + pod.Name + "' is waiting for reason '" + containerStatus.State.Waiting.Reason + "'", false)
            }
        }
    }
}


func CheckCertManager() {
    // Check cert-manager namespace
    namespaces, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        common.LogError(err.Error())
    }

    for _, namespace := range namespaces.Items {
        if namespace.Name == "cert-manager" {
            common.PrettyPrintStr("cert-manager", true, "available")
        }
    }

    // Get a list of cert-manager.io/Certificate resources
    d, err := clientset.RESTClient().Get().RequestURI("/apis/cert-manager.io/v1/certificates").DoRaw(context.Background())
    if err != nil {
        common.LogError(err.Error())
    }

    // Parse JSON
    var certManager CertManager
    err = json.Unmarshal(d, &certManager)

    if err != nil {
        common.LogError(err.Error())
    }

    for _, item := range certManager.Items {
        for _, condition := range item.Status.Conditions {
            if condition.Type == "Ready" {
                if condition.Status != "True" {
                    common.PrettyPrintStr(item.Metadata.Name, false, "Ready")
                    common.AlarmCheckDown(item.Metadata.Name + "_ready", "Certificate named '" + item.Metadata.Name + "' is not Ready", false)
                } else {
                    common.AlarmCheckUp(item.Metadata.Name + "_ready", "Certificate named '" + item.Metadata.Name + "' is now Ready", false)
                }
            }
        }
    }
}

func CheckKubeVip() {
    // Check if kube-vip pods exists on kube-system namespace
    pods, err := clientset.CoreV1().Pods("kube-system").List(context.TODO(), metav1.ListOptions{})
    if err != nil {
        common.LogError(err.Error())
    }

    kubeVipExists := false

    for _, pod := range pods.Items {
    if strings.Contains(pod.Name, "kube-vip") {
            kubeVipExists = true
            break
        }
    }

    if kubeVipExists {
        common.PrettyPrintStr("kube-vip", true, "available")
        for _, floatingIp := range K8sHealthConfig.K8s.Floating_Ips {
            pinger, err := probing.NewPinger(floatingIp)
            if err != nil {
                common.LogError(err.Error())
            }
            pinger.Count = 1
            pinger.Timeout = 10 * time.Second

            err = pinger.Run()
            if err != nil {
                common.PrettyPrintStr(floatingIp, false, "available")
                common.AlarmCheckDown("floating_unexpected_ " + floatingIp, "Floating IP " + floatingIp + " is not available", false)
            } else {
                common.AlarmCheckUp("floating_unexpected_ " + floatingIp, "Floating IP " + floatingIp + " is available", false)
            }
        }
    } else {
        common.PrettyPrintStr("kube-vip", false, "available")
    }
}


func CheckClusterApiCert() {
    crtFile := "/var/lib/rancher/rke2/server/tls/serving-kube-apiserver.crt"

    if common.FileExists(crtFile) {
        common.PrettyPrintStr(filepath.Base(crtFile), true, "available")
    } else {
        common.PrettyPrintStr(filepath.Base(crtFile), false, "available")
        return
    }

    // Get the expiration date of the certificate
    certFile, err := os.ReadFile(crtFile) 
    if err != nil {
        common.LogError(err.Error())
    }

    block, _ := pem.Decode(certFile)
    if block == nil {
        common.LogError("failed to parse certificate PEM")
    }

    cert, err := x509.ParseCertificate(block.Bytes)

    if err != nil {
        common.LogError(err.Error())
    }

    // Check if the certificate is expired
    if cert.NotAfter.Before(time.Now()) {
        common.PrettyPrintStr("kube-apiserver", true, "expired")
        common.AlarmCheckDown("kube-apiserver_expired", "kube-apiserver certificate is expired", false)
    } else {
        common.PrettyPrintStr("kube-apiserver", true, "not expired")
        common.AlarmCheckUp("kube-apiserver_expired", "kube-apiserver certificate is now valid", false)
    }
}

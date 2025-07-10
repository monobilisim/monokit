#!/bin/sh
set -e

sudo mkdir -p /etc/mono
sudo touch /etc/mono/k8s.yaml
export MONOKIT_NOCOLOR=1

./bin/monokit k8sHealth > out.log

# Function to check grep and show output on failure
check_grep() {
    if ! cat out.log | grep "$1" > /dev/null; then
        echo "Failed to find: $1"
        echo "Full output:"
        cat out.log
        exit 1
    fi
}

check_grep "Master Nodes Ready.*less than 1"
check_grep "Worker Nodes Ready.*more than 0"
check_grep "Manifest Dir.*is Not Found"
check_grep "Cert-Manager Status"
check_grep "Namespace (cert-manager) is Present"
check_grep "Kube-VIP Pods.*is Not Detected"
check_grep "API Certificate Check is Error"
check_grep "Cluster API server certificate file not found"
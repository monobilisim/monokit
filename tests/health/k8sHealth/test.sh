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

check_grep "Master Nodes Ready.*less than"
check_grep "Ingress Nginx Config.*not available"
check_grep "the server could not find the requested resource"
check_grep "Kube-VIP.*not available"
check_grep "Cluster API Certificate.*not available"
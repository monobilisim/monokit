#!/bin/sh
sudo mkdir -p /etc/mono
sudo touch /etc/mono/k8s.yaml
./bin/monokit k8sHealth
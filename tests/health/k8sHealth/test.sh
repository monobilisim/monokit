#!/bin/sh
set -e

sudo mkdir -p /etc/mono
sudo touch /etc/mono/k8s.yaml
export MONOKIT_NOCOLOR=1

./bin/monokit k8sHealth > out.log

cat out.log | grep "is Ready"
cat out.log | grep "rke2-ingress-nginx-config.yaml is not available"
cat out.log | grep "the server could not find the requested resource"
cat out.log | grep "kube-vip is not available"
cat out.log | grep "serving-kube-apiserver.crt is not available"
#!/bin/sh
sudo apt update
sudo apt upgrade -y 
sudo apt install curl -y

curl -sfL https://get.k3s.io | sudo sh -

if systemctl is-active --quiet k3s; then
  echo "K3s installation successful!"
else
  echo "K3s installation failed. Please check the logs for more details."
  exit 1
fi

sleep 60
systemctl status k3s


sudo mkdir /root/.kube/
sudo cp /etc/rancher/k3s/k3s.yaml /root/.kube/config
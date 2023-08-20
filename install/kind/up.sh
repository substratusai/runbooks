#!/bin/bash

set -e
set -u

# This is a workaround for GPU support in kind:
# https://github.com/kubernetes-sigs/kind/pull/3257#issuecomment-1607287275
# TODO: add a check if nvidia is default runtime
# TODO: add all GPU hacks behind a conditional
nvidia_config_file="/etc/nvidia-container-runtime/config.toml"
if [ -e "${nvidia_config_file}" ]; then

sudo sed -i '/accept-nvidia-visible-devices-as-volume-mounts/c\accept-nvidia-visible-devices-as-volume-mounts = true' ${nvidia_config_file}
fi


kind create cluster --name substratus --config - <<EOF
apiVersion: kind.x-k8s.io/v1alpha4
kind: Cluster
nodes:
- role: control-plane
  image: kindest/node:v1.27.3@sha256:3966ac761ae0136263ffdb6cfd4db23ef8a83cba8a463690e98317add2c9ba72
  extraPortMappings:
  - containerPort: 30080
    hostPort: 30080
  # part of GPU workaround
  extraMounts:
    - hostPath: /dev/null
      containerPath: /var/run/nvidia-container-devices/all
EOF


set -x
# The nvidia operator needs the below symlink
# https://github.com/NVIDIA/nvidia-docker/issues/614#issuecomment-423991632
docker exec -ti substratus-control-plane ln -s /sbin/ldconfig /sbin/ldconfig.real || true

helm repo add nvidia https://helm.ngc.nvidia.com/nvidia || true
helm repo update
helm install --wait --generate-name \
     -n gpu-operator --create-namespace \
     nvidia/gpu-operator --set driver.enabled=false

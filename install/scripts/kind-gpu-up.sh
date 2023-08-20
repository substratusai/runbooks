#!/bin/bash

set -e
set -u

# This is a workaround for GPU support in kind:
# https://github.com/kubernetes-sigs/kind/pull/3257#issuecomment-1607287275
if ! docker info | grep -q "Default Runtime: nvidia"; then
  echo "nvidia needs to be the default runtime to configure kind with GPU support."
  echo "See steps here for more details: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/user-guide.html#adding-the-nvidia-runtime"
  echo "In most cases running the follow steps will configure nvidia as the default runtime:"
  echo "sudo nvidia-ctk runtime configure --runtime=docker"
  echo "sudo systemctl restart docker"
fi

nvidia_config_file="/etc/nvidia-container-runtime/config.toml"
if [ -e "${nvidia_config_file}" ]; then
  sudo sed -i '/accept-nvidia-visible-devices-as-volume-mounts/c\accept-nvidia-visible-devices-as-volume-mounts = true' ${nvidia_config_file}
else
  echo "error: Missing file ${nvidia_config_file}, which likely means you have not configured the nvidia container runtime."
  echo "You can follow the install steps here to install the NVIDIA container toolkit: https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html"
  exit 1
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
  # required for GPU workaround
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

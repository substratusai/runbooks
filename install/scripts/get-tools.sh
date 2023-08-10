#!/bin/bash

terraform_version="1.4.5"

tempout=$(mktemp -d)

# Determine platform and architecture
arch=$(uname -m)
platform=$(uname -s | tr '[:upper:]' '[:lower:]')

if [[ "$arch" = "aarch64" || "$arch" = "arm64" ]]; then
  awscli_arch=aarch64
  terraform_arch=arm64
  platform_arch=${platform}_arm64
elif [ "$arch" = "x86_64" ]; then
  awscli_arch=x86_64
  terraform_arch=amd64
  platform_arch=${platform}_amd64
else
  echo "Unsupported architecture"
  exit 1
fi

# install all our common tools
if [ "${platform}" == "linux" ]; then
  DEBIAN_FRONTEND="noninteractive" \
    apt-get update
  apt-get install -y \
    gnupg \
    software-properties-common \
    unzip \
    curl \
    git \
    python3-venv \
    gettext-base
elif [ "${platform}" == "darwin" ]; then
  brew install \
    gnupg \
    unzip \
    curl \
    git \
    gettext
else
  echo "Unsupported platform"
  exit 1
fi

install_awscli() {
  curl "https://s3.amazonaws.com/aws-cli/awscli-bundle.zip" -o "${tempout}/awscli-bundle.zip"
  unzip "${tempout}/awscli-bundle.zip" -d ${tempout}
  python3 "${tempout}/awscli-bundle/install" -i /usr/local/aws -b /usr/local/bin/aws
}

install_eksctl() {
  curl -sL "https://github.com/eksctl-io/eksctl/releases/latest/download/eksctl_${platform_arch}.tar.gz" \
    -o ${tempout}/eksctl.tar.gz
  tar -xzf ${tempout}/eksctl.tar.gz -C /tmp
  (mv /tmp/eksctl /usr/local/bin || sudo mv /tmp/eksctl /usr/local/bin)
}

install_terraform() {
  curl https://releases.hashicorp.com/terraform/${terraform_version}/terraform_${terraform_version}_${platform}_${terraform_arch}.zip \
    -o ${tempout}/terraform.zip
  unzip ${tempout}/terraform.zip -d ${tempout}
  (mv ${tempout}/terraform /usr/local/bin/ || sudo mv ${tempout}/terraform /usr/local/bin/)
}

install_gcloud() {
  curl https://dl.google.com/dl/cloudsdk/release/google-cloud-sdk.tar.gz \
    -o ${tempout}/google-cloud-sdk.tar.gz
  mkdir -p /usr/local/gcloud
  tar -C /usr/local/gcloud -xvf ${tempout}/google-cloud-sdk.tar.gz
  /usr/local/gcloud/google-cloud-sdk/install.sh
  gcloud components install gke-gcloud-auth-plugin kubectl
}

install_helm() {
  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 \
    -o ${tempout}/get_helm.sh
  chmod 700 ${tempout}/get_helm.sh
  ${tempout}/get_helm.sh
}

if ! command -v aws &>/dev/null; then install_awscli; fi
if ! command -v eksctl &>/dev/null; then install_eksctl; fi
if ! command -v terraform &>/dev/null; then install_terraform; fi
if ! command -v gcloud &>/dev/null; then install_gcloud; fi
if ! command -v kubectl &>/dev/null; then install_gcloud; fi
if ! command -v helm &>/dev/null; then install_helm; fi

rm -r ${tempout}

echo "Installation complete!"

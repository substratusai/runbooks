FROM ubuntu:23.04

WORKDIR /workspace
# Determine platform and architecture
RUN ARCH=$(uname -m) && \
  PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]') && \
  if [ "$ARCH" = "aarch64" ]; then \
  echo "AWSCLI_ARCH=aarch64" >> /etc/environment; \
  echo "TERRAFORM_ARCH=arm64" >> /etc/environment; \
  echo "PLATFORM_ARCH=${PLATFORM}_arm64" >> /etc/environment; \
  elif [ "$ARCH" = "x86_64" ]; then \
  echo "AWSCLI_ARCH=x86_64" >> /etc/environment; \
  echo "TERRAFORM_ARCH=amd64" >> /etc/environment; \
  echo "PLATFORM_ARCH=${PLATFORM}_amd64" >> /etc/environment; \
  else \
  echo "Unsupported architecture"; \
  exit 1; \
  fi

# Source the environment file so that the variable is available in the current shell
SHELL ["/bin/bash", "-c"]
RUN source /etc/environment

# Common
RUN DEBIAN_FRONTEND="noninteractive" \
  apt-get update && \
  apt-get install -y \
  gnupg \
  software-properties-common \
  unzip \
  wget \
  curl \
  git \
  tzdata \
  keyboard-configuration

# AWS CLI
RUN source /etc/environment && \
  curl "https://awscli.amazonaws.com/awscli-exe-linux-${AWSCLI_ARCH}.zip" -o "awscliv2.zip" && \
  unzip awscliv2.zip && \
  ./aws/install

# eksctl
RUN source /etc/environment && \
  curl -sLO "https://github.com/eksctl-io/eksctl/releases/latest/download/eksctl_${PLATFORM_ARCH}.tar.gz" && \
  tar -xzf eksctl_${PLATFORM_ARCH}.tar.gz -C /tmp && rm eksctl_${PLATFORM_ARCH}.tar.gz && \
  mv /tmp/eksctl /usr/local/bin

# Terraform
RUN source /etc/environment && \
  wget https://releases.hashicorp.com/terraform/1.4.5/terraform_1.4.5_linux_${TERRAFORM_ARCH}.zip && \
  unzip terraform_1.4.5_linux_${TERRAFORM_ARCH}.zip && \
  mv terraform /usr/local/bin/ && \
  terraform --version

# Google Cloud (gcloud)
RUN curl https://dl.google.com/dl/cloudsdk/release/google-cloud-sdk.tar.gz > /tmp/google-cloud-sdk.tar.gz && \
  mkdir -p /usr/local/gcloud \
  && tar -C /usr/local/gcloud -xvf /tmp/google-cloud-sdk.tar.gz \
  && /usr/local/gcloud/google-cloud-sdk/install.sh
ENV PATH $PATH:/usr/local/gcloud/google-cloud-sdk/bin
RUN gcloud --version
RUN gcloud components install gke-gcloud-auth-plugin kubectl

# Helm
RUN curl -fsSL -o /tmp/get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
RUN chmod 700 /tmp/get_helm.sh
RUN /tmp/get_helm.sh

# Local files
COPY terraform terraform
COPY kubernetes kubernetes
COPY scripts scripts
ENV PATH $PATH:/workspace/scripts

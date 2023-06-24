FROM ubuntu:23.04

# Common
RUN apt-get update && \
  apt-get install -y \
  gnupg \
  software-properties-common \
  unzip \
  wget \
  curl \
  git

# Terraform
RUN wget https://releases.hashicorp.com/terraform/1.4.5/terraform_1.4.5_linux_amd64.zip
RUN unzip terraform_1.4.5_linux_amd64.zip
RUN mv terraform /usr/local/bin/
RUN terraform --version

# Google Cloud (gcloud)
RUN curl https://dl.google.com/dl/cloudsdk/release/google-cloud-sdk.tar.gz > /tmp/google-cloud-sdk.tar.gz
RUN mkdir -p /usr/local/gcloud \
  && tar -C /usr/local/gcloud -xvf /tmp/google-cloud-sdk.tar.gz \
  && /usr/local/gcloud/google-cloud-sdk/install.sh
ENV PATH $PATH:/usr/local/gcloud/google-cloud-sdk/bin
RUN gcloud --version
RUN gcloud components install gke-gcloud-auth-plugin

# Kubectl
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl
RUN chmod +x ./kubectl
RUN mv ./kubectl /usr/local/bin

# Helm
RUN curl -fsSL -o /tmp/get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
RUN chmod 700 /tmp/get_helm.sh
RUN /tmp/get_helm.sh

# Local files
COPY terraform /terraform
COPY commands /commands
ENV PATH $PATH:/commands

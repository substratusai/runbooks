FROM ubuntu:23.04

ENV PATH $PATH:/workspace/scripts:/usr/local/gcloud/google-cloud-sdk/bin
WORKDIR /workspace

COPY scripts scripts

RUN scripts/get-tools.sh

COPY terraform terraform
COPY kubernetes kubernetes

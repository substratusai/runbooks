#!/usr/bin/env bash
set -xe

version=v0.8.0
os=$(uname -s)
arch=$(uname -m | sed 's/aarch64/arm64/g')

release_url="https://github.com/substratusai/substratus/releases/download/$version/kubectl-plugins-$os-$arch.tar.gz"

wget -qO- $release_url | tar zxv --directory /tmp
chmod +x /tmp/kubectl-applybuild
chmod +x /tmp/kubectl-notebook
mv /tmp/kubectl-applybuild /usr/local/bin/ || sudo mv /tmp/kubectl-applybuild /usr/local/bin/
mv /tmp/kubectl-notebook /usr/local/bin/ || sudo mv /tmp/kubectl-notebook /usr/local/bin/
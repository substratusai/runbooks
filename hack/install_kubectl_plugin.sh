#!/usr/bin/env bash
set -xe

REPO='https://github.com/substratusai/substratus'
LATEST_RELEASE=$(curl ${REPO}/releases -s |
  grep 'Link--primary' |
  head -n1 |
  perl -n -e '/v([0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+)?)/ && print $&')
OS=$(uname -s)
ARCH=$(uname -m | sed 's/aarch64/arm64/g')
LATEST_ARTIFACT_URL=$(echo $LATEST_RELEASE | awk -v repo=$REPO -v os=$OS -v arch=$ARCH -v release=$LATEST_RELEASE '{print repo "/releases/download/" release "/kubectl-open-notebook_" os "_" arch ".tar.gz"}')

wget -qO- ${LATEST_ARTIFACT_URL} | tar zxv
chmod +x kubectl-open-notebook
mv kubectl-open-notebook /usr/local/bin/ || sudo mv kubectl-open-notebook /usr/local/bin/

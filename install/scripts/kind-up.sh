#!/bin/bash

set -e
set -u

kind create cluster --name substratus --config kind-config.yaml
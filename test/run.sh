#!/bin/bash

set -e
set -x

cloud=$1
repo=$(git rev-parse --show-toplevel)
example="facebook-opt-125m"

if [[ -z "$cloud" ]]; then
	echo "Must provide <cloud> arg"
	exit 1
fi

echo "Running test for cloud: $cloud"

function down {
	if [ "${DOWN}" == "no" ]; then
		echo "Skipping DOWN..."
	else
		echo "Running DOWN..."
		./install/${cloud}/down.sh
	fi
}
trap down EXIT

if [ "${UP}" == "no" ]; then
	echo "Skipping UP..."
else
	./install/${cloud}/up.sh
fi

# Install Substratus
kubectl apply -k ${repo}/test/install/${cloud}

# Import a Model
kubectl apply -f ${repo}/examples/${example}/base-model.yaml

# Serve the Model
kubectl apply -f ${repo}/examples/${example}/base-server.yaml

# Wait until both are ready
# TODO: Consider adding common "Ready" condition to make this check easier.
kubectl wait --for=condition=modelled models --all --timeout 600s
kubectl wait --for=condition=deployed servers --all --timeout 600s

# Forward ports to localhost
kubectl port-forward service/$example-server 8080:8080 &
port_forward_pid=$!

function stop_port_forward {
	kill $port_forward_pid
	# Allow for the port-forward to stop
	sleep 1
	# Trap will only call 1 function, so we must call the previous trap function
	down
}
trap stop_port_forward EXIT

# Wait for port-forward to be ready (client-side)
sleep 3

# Send example request
curl http://localhost:8080/v1/completions \
	-H "Content-Type: application/json" \
	-d '{ \
    "prompt": "What is your favorite color? ", \
    "max_tokens": 3\
  }'

kill $port_forward_pid

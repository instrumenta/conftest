#!/bin/bash

# This script validates that bundles can be pushed and pulled
# from a local registry using the registry image from DockerHub
# https://hub.docker.com/_/registry

trap 'cleanup' ERR SIGTERM SIGINT

CONFTEST="./conftest"
CONTAINER_NAME="conftest-push-pull-e2e"

function cleanup() {
    docker rm $CONTAINER_NAME -f
    rm -r tmp
}

docker run --name $CONTAINER_NAME -d registry
if [ $? != 0 ]; then
    echo "ERROR RUNNING TEST CONTAINER. IS DOCKER INSTALLED?"
    exit 1
fi

$CONFTEST push localhost:5000/test -p examples/data
if [ $? != 0 ]; then
    echo "ERROR PUSHING BUNDLE"
    exit 1
fi

$CONFTEST pull localhost:5000/test -p tmp
if [ $? != 0 ]; then
    echo "ERROR PULLING BUNDLE"
    exit 1
fi

$CONFTEST verify -p tmp/examples/data/policy -d tmp/examples/data/exclusions tmp/examples/data/service.yaml
if [ $? != 0 ]; then
    echo "POLICIES WERE NOT SUCCESSFULLY VERIFIED"
    exit 1
fi

cleanup
exit 0

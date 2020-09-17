#!/usr/bin/env bash

# kubectl-conftest allows for testing resources in your cluster using Open Policy Agent
# It uses the conftest utility and expects to find associated policy files in
# a directory called policy

# Check if a specified command exists on the path and is executable
function check_command () {
    if ! [[ -x $(command -v "$1") ]] ; then
        echo "$1 not installed"
        exit 1
    fi
}

function usage () {
    echo "A Kubectl plugin for using Conftest to test objects in Kubernetes using Open Policy Agent"
    echo
    echo "See https://github.com/open-policy-agent/conftest for more information"
    echo
    echo "Usage:"
    echo "   conftest kubectl (TYPE[.VERSION][.GROUP] [NAME] | TYPE[.VERSION][.GROUP]/NAME)"
}

# Check the required commands are available on the PATH
check_command "kubectl"
check_command "conftest"

if [[ ($# -eq 0) || ($1 == "--help") || ($1 == "-h") ]]; then
    # No commands or the --help flag passed and we'll show the usage instructions
    usage
elif [[ ($# -eq 1) && $1 =~ ^[a-z\.]+$ ]]; then
    # If we have one argument we get the list of objects from kubectl and pass the items to conftest
    check_command "jq"
    if output=$(kubectl get "$1" -o json); then
        echo "$output" | jq .items | conftest test -
    fi
elif [[ ($# -eq 1 ) ]]; then
    # Support the / variant for getting an individual resource
    if output=$(kubectl get "$1" -o json); then
        echo "Testing $1"
        echo "$output" | conftest test -
    fi
elif [[ ($# -eq 2 ) && $1 =~ ^[a-z]+$ ]]; then
    # if we have two arguments then we assume the first is the type and the second the resource name
    if output=$(kubectl get "$1" "$2" -o json); then
        echo "Testing $1/$2"
        echo "$output" | conftest test -
    fi
else
    echo "Please check the arguments to kubectl conftest"
    echo
    usage
    exit 1
fi

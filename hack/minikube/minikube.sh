#!/bin/bash

# Provide a "yes" repsonse to the warning prompt as a command line arg.
YES=""

OS="$(uname -s)"
info_message() {
    if [ -z "${1}" ]; then
        echo "info_message() requires a message"
        exit 1
    fi
    echo -e "[\033[0;34m ACTION \033[0m] ${1}"
}

pass_message() {
    if [ -z "${1}" ]; then
        echo "pass_message() requires a message"
        exit 1
    fi
    echo -e "[\033[0;32m PASSED \033[0m] ${1}"
}

error_message() {
    if [ -z "${1}" ]; then
        echo "error_message() requires a message"
        exit 1
    fi
    if [ -n "$1" ]; then
        echo -e "[\033[0;31m FAILED \033[0m] ] ${1}"
    fi
}

function up() {
    uri_ip=$1

    if ! [ -x "$(command -v minikube)" ]; then
        error_message "Minikube not found. Please install Minikube."
        exit 1
    fi
    if ! [ -x "$(command -v kubectl)" ]; then
        error_message "Kubectl not found. Please install kubectl."
        exit 1
    fi
    info_message "Creating minikube cluster..."
    minikube start --addons=ingress
    if [ "$?" != "0" ]; then 
        error_message "Failed to create minikube cluster."
        exit 1
    fi


    info_message "Creating Apex namespace..."
    kubectl create namespace apex

    info_message "Deploying Apex stack..."
    sed -e "s|UI_URL_VALUE|http://${uri_ip}|g" ../../deploy/apex.yaml | kubectl apply -f -

    info_message "Wait for Apex stack readiness..."
    kubectl wait --for=condition=Ready pods --all -n apex --timeout=120s
    
    info_message "Deploying Apex Ingress..."
    sed  '/<HOST_DNS>/d' ../../deploy/apex-ingress.yaml | kubectl apply -f -
    pass_message "All the resources are deployed on the Minikube cluster"
}

function down() {
    if ! [ -x "$(command -v minikube)" ]; then
        error_message "Minikube not found. Are you sure you are running local cluster?"
        exit 1
    fi
    profile=$(minikube profile | grep -w "minikube")
    if [ "$profile" == "minikube" ]; then
        info_message "Deleting minikube cluster..."
        minikube stop; minikube delete
    else
        info_message "Minikube cluster is not running."
        exit 1
    fi
}

function help() {
    printf "\n"
    printf "Usage: %s [-u:dh]\n" "$0"
    printf "\t-u <ingress-ip> Create Minikube cluster and install Apex stack.
            ingress-ip: IP of the machine where minikube is running.\n"
    printf "\t-d Delete Minikube cluster \n"
    printf "\t-h help\n"
    exit 1
}

while getopts "u:dh" opt; do
    case $opt in
        u ) 
            up ${OPTARG};;
        d ) down;;
        h ) help
        exit 0;;
        *) help
        exit 1;;
    esac
done
if [ $# -eq 0 ]; then 
    help;exit 0 
fi

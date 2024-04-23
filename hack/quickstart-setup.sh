#!/bin/bash

#
# Copyright 2021 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -e pipefail

containerRuntime() {
  local container_runtime=""
  if command -v docker &>/dev/null; then
    container_runtime="docker"
  elif command -v podman &>/dev/null; then
    container_runtime="podman"
  else
    echo "Neither Docker nor Podman is installed. Exiting..."
    exit 1
  fi
  echo "$container_runtime"
}

dockerBinCmd() {
  local network=""
  if [ ! -z "${KIND_CLUSTER_DOCKER_NETWORK}" ]; then
    network=" --network ${KIND_CLUSTER_DOCKER_NETWORK}"
  fi

  echo "$CONTAINER_RUNTIME_BIN run --rm -u $UID -v ${TMP_DIR}:${TMP_DIR}${network} -e KUBECONFIG=${TMP_DIR}/kubeconfig --entrypoint=$1 $TOOLS_IMAGE"
}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color
BOLD='\033[1m'
INFO="${BOLD}${YELLOW}INFO:${NC}"
SUCCESS="${GREEN}✓${NC}"
FAILURE="${RED}✗${NC}"

if [ -z $KUADRANT_ORG ]; then
  KUADRANT_ORG=${KUADRANT_ORG:="kuadrant"}
fi
if [ -z $KUADRANT_REF ]; then
  KUADRANT_REF=${KUADRANT_REF:="main"}
fi
if [ -z $MGC_REF ]; then
  MGC_REF=${MGC_REF:="main"}
fi

export TOOLS_IMAGE=quay.io/kuadrant/mgc-tools:latest
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export TMP_DIR=$SCRIPT_DIR/tmp/mgc
export CONTAINER_RUNTIME_BIN=$(containerRuntime)
export KIND_BIN=kind
export HELM_BIN=helm
export KUSTOMIZE_BIN=$(dockerBinCmd "kustomize")

# Default to 1 cluster if KUADRANT_CLUSTER_COUNT is not set
KUADRANT_CLUSTER_COUNT=${KUADRANT_CLUSTER_COUNT:-1}

YQ_BIN=$(dockerBinCmd "yq")

KUADRANT_REPO="github.com/${KUADRANT_ORG}/kuadrant-operator.git"
KUADRANT_REPO_RAW="https://raw.githubusercontent.com/${KUADRANT_ORG}/kuadrant-operator/${KUADRANT_REF}"
KUADRANT_DEPLOY_KUSTOMIZATION="${KUADRANT_REPO}/config/deploy?ref=${KUADRANT_REF}"
KUADRANT_GATEWAY_API_KUSTOMIZATION="${KUADRANT_REPO}/config/dependencies/gateway-api?ref=${KUADRANT_REF}"
KUADRANT_ISTIO_KUSTOMIZATION="${KUADRANT_REPO}/config/dependencies/istio/sail?ref=${KUADRANT_REF}"
KUADRANT_CERT_MANAGER_KUSTOMIZATION="${KUADRANT_REPO}/config/dependencies/cert-manager?ref=${KUADRANT_REF}"
KUADRANT_METALLB_KUSTOMIZATION="${KUADRANT_REPO}/config/metallb?ref=${KUADRANT_REF}"
MGC_REPO="github.com/${KUADRANT_ORG}/multicluster-gateway-controller.git"
MGC_ISTIO_KUSTOMIZATION="${MGC_REPO}/config/istio?ref=${MGC_REF}"

# Make temporary directory
mkdir -p ${TMP_DIR}

KUADRANT_CLUSTER_NAME=kuadrant-local
KUADRANT_NAMESPACE=kuadrant-system

info() {
  echo -e "${INFO} $1"
}

success() {
  echo -e "${SUCCESS} $1"
}

error() {
  echo -e "${FAILURE} $1"
}

check_dependencies() {
  # Check for Docker or Podman
  if ! command -v docker &>/dev/null && ! command -v podman &>/dev/null; then
    error "Neither docker nor podman could be found. Please install Docker or Podman."
    exit 1
  fi

  # Check for other dependencies
  for cmd in kind kubectl; do
    if ! command -v $cmd &>/dev/null; then
      error "Error: $cmd could not be found. Please install $cmd."
      exit 1
    fi
  done

  success "All dependencies are installed."
}

# Generate MetalLB IpAddressPool for a given network
generate_ip_address_pool() {
  local network_name="$1"
  local script_path="${SCRIPT_DIR}/../utils/docker-network-ipaddresspool.sh"

  # interactively or piped
  if [ -t 0 ]; then
    # interactively
    if [ -f "$script_path" ]; then
      bash "$script_path" "$network_name"
    else
      echo "Script file not found at $script_path" >&2
      return 1
    fi
  else
    # piped
    curl -s "${KUADRANT_REPO_RAW}/utils/docker-network-ipaddresspool.sh" | bash -s -- "$network_name"
  fi
}

requiredENV() {
  info "Configuring DNS provider environment variables... 🛰️"
  info "You have chosen to set up a DNS provider, which is required for using Kuadrant's DNSPolicy API."
  info "Supported DNS providers are AWS Route 53 and Google Cloud DNS."

  # Read directly from the terminal, ensuring it can handle piped script execution
  read -r -p "Please enter 'aws' for AWS Route 53, or 'gcp' for Google Cloud DNS: " DNS_PROVIDER </dev/tty

  if [[ "$DNS_PROVIDER" =~ ^(aws|gcp)$ ]]; then
    info "You have selected the $DNS_PROVIDER DNS provider."
  else
    error "Invalid input. Supported providers are 'aws' and 'gcp' only. Exiting."
    exit 1
  fi
  export DNS_PROVIDER

  if [[ "$DNS_PROVIDER" == "aws" ]]; then
    if [[ -z "${KUADRANT_AWS_ACCESS_KEY_ID}" ]]; then
      echo "Enter an AWS access key ID for an account where you have access to AWS Route 53:"
      read -r KUADRANT_AWS_ACCESS_KEY_ID </dev/tty
      echo "export KUADRANT_AWS_ACCESS_KEY_ID for future executions of the script to skip this step"
    fi

    if [[ -z "${KUADRANT_AWS_SECRET_ACCESS_KEY}" ]]; then
      echo "Enter the corresponding AWS secret access key for the AWS access key ID entered above:"
      read -r KUADRANT_AWS_SECRET_ACCESS_KEY </dev/tty
      echo "export KUADRANT_AWS_SECRET_ACCESS_KEY for future executions of the script to skip this step"
    fi

    if [[ -z "${KUADRANT_AWS_REGION}" ]]; then
      echo "Enter an AWS region (e.g. eu-west-1) for an Account where you have access to AWS Route 53:"
      read -r KUADRANT_AWS_REGION </dev/tty
      echo "export KUADRANT_AWS_REGION for future executions of the script to skip this step"
    fi

    if [[ -z "${KUADRANT_AWS_DNS_PUBLIC_ZONE_ID}" ]]; then
      echo "Enter the Public Zone ID of your Route53 zone:"
      read -r KUADRANT_AWS_DNS_PUBLIC_ZONE_ID </dev/tty
      echo "export KUADRANT_AWS_DNS_PUBLIC_ZONE_ID for future executions of the script to skip this step"
    fi

    if [[ -z "${KUADRANT_ZONE_ROOT_DOMAIN}" ]]; then
      echo "Enter the root domain of your Route53 hosted zone (e.g. www.example.com):"
      read -r KUADRANT_ZONE_ROOT_DOMAIN </dev/tty
      echo "export KUADRANT_ZONE_ROOT_DOMAIN for future executions of the script to skip this step"
    fi
  else
    if [[ -z "${GOOGLE}" ]]; then
      echo "Enter either credentials created either by CLI or by service account (Please make sure the credentials provided are in JSON format)"
      read -r GOOGLE </dev/tty
      echo "export GOOGLE for future executions of the script to skip this step"
    fi
    if ! jq -e . <<<"$GOOGLE" >/dev/null 2>&1; then
      echo "Credentials provided is not in JSON format"
      exit 1
    fi

    if [[ -z "${PROJECT_ID}" ]]; then
      echo "Enter the project id for your GCP Cloud DNS:"
      read -r PROJECT_ID </dev/tty
      echo "export PROJECT_ID for future executions of the script to skip this step"
    fi

    if [[ -z "${ZONE_DNS_NAME}" ]]; then
      echo "Enter the DNS name for your GCP Cloud DNS:"
      read -r ZONE_DNS_NAME </dev/tty
      echo "export ZONE_DNS_NAME for future executions of the script to skip this step"
    fi

    if [[ -z "${ZONE_NAME}" ]]; then
      echo "Enter the Zone name for your GCP Cloud DNS:"
      read -r ZONE_NAME </dev/tty
      echo "export ZONE_NAME for future executions of the script to skip this step"
    fi
  fi
}

setupAWSProvider() {
  local namespace="$1"
  if [ -z "$1" ]; then
    namespace="multi-cluster-gateways"
  fi
  if [ "$KUADRANT_AWS_ACCESS_KEY_ID" == "" ]; then
    echo "KUADRANT_AWS_ACCESS_KEY_ID is not set"
    exit 1
  fi

  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${KIND_CLUSTER_PREFIX}aws-credentials
  namespace: ${namespace}
type: "kuadrant.io/aws"
stringData:
  AWS_ACCESS_KEY_ID: ${KUADRANT_AWS_ACCESS_KEY_ID}
  AWS_SECRET_ACCESS_KEY: ${KUADRANT_AWS_SECRET_ACCESS_KEY}
  AWS_REGION: ${KUADRANT_AWS_REGION}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${KIND_CLUSTER_PREFIX}controller-config
  namespace: ${namespace}
data:
  AWS_DNS_PUBLIC_ZONE_ID: ${KUADRANT_AWS_DNS_PUBLIC_ZONE_ID}
  ZONE_ROOT_DOMAIN: ${KUADRANT_ZONE_ROOT_DOMAIN}
  LOG_LEVEL: "${LOG_LEVEL}"
---
apiVersion: kuadrant.io/v1alpha1
kind: ManagedZone
metadata:
  name: ${KIND_CLUSTER_PREFIX}dev-mz
  namespace: ${namespace}
spec:
  id: ${KUADRANT_AWS_DNS_PUBLIC_ZONE_ID}
  domainName: ${KUADRANT_ZONE_ROOT_DOMAIN}
  description: "Dev Managed Zone"
  dnsProviderSecretRef:
    name: ${KIND_CLUSTER_PREFIX}aws-credentials
EOF
}

setupGCPProvider() {
  local namespace="$1"
  if [ -z "$1" ]; then
    namespace="multi-cluster-gateways"
  fi
  kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: ${KIND_CLUSTER_PREFIX}gcp-credentials
  namespace: ${namespace}
type: "kuadrant.io/gcp"
stringData:
  GOOGLE: '${GOOGLE}'
  PROJECT_ID: ${PROJECT_ID}
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: ${KIND_CLUSTER_PREFIX}controller-config
  namespace: ${namespace}
data:
  ZONE_DNS_NAME: ${ZONE_DNS_NAME}
  ZONE_NAME: ${ZONE_NAME}
  LOG_LEVEL: "${LOG_LEVEL}"
---
apiVersion: kuadrant.io/v1alpha1
kind: ManagedZone
metadata:
  name: ${KIND_CLUSTER_PREFIX}dev-mz
  namespace: ${namespace}
spec:
  id: ${ZONE_NAME}
  domainName: ${ZONE_DNS_NAME}
  description: "Dev Managed Zone"
  dnsProviderSecretRef:
    name: ${KIND_CLUSTER_PREFIX}gcp-credentials
EOF
}

setupClusterIssuer() {
  info "Creating a default ClusterIssuer on ${clusterName}... 🔒"
  clusterName=${1}
  kubectl config use-context kind-${clusterName}

  echo "woop woop kind-${clusterName}"
  kubectl --context kind-${clusterName} apply -f - <<EOF
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: kuadrant-operator-glbc-ca
spec:
  selfSigned: {}
EOF
  kubectl --context kind-${clusterName} wait --for=condition=available --timeout=300s deployment --all -n cert-manager
  success "ClusterIssuer created on ${clusterName}"
}

postSetup() {
  clusterName=${1}
  namespace=${2}
  kubectl config use-context kind-${clusterName}
  info "Running post-deployment setup on ${clusterName} ⌛"

  case $DNS_PROVIDER in
  aws)
    echo "Setting up an AWS Route 53 DNS provider"
    setupAWSProvider ${namespace}
    ;;
  gcp)
    echo "Setting up a Google Cloud DNS provider"
    setupGCPProvider ${namespace}
    ;;
  *)
    echo "Unknown DNS provider"
    exit
    ;;
  esac
}

info "📘 Welcome to the Kuadrant Quick Start setup process"

info "This script will guide you through setting up a local Kubernetes cluster with the following components:"
info "  - Docker or Podman (Container Runtime)"
info "  - kind (Kubernetes IN Docker)"
info "  - Kuadrant and its dependencies, including:"
info "      * Gateway API"
info "      * Istio"
info "      * Cert-Manager"
info "      * MetalLB"
info "  - Optional DNS provider setup for Kuadrant's DNSPolicy API"

info "Please ensure you have an internet connection and local admin access to perform installations."

read -r -p "Are you ready to begin? (y/n) " yn </dev/tty

case $yn in
[Yy]*)
  echo "Starting the setup process..."
  ;;
[Nn]*)
  echo "Setup canceled by user."
  exit
  ;;
*)
  echo "Invalid input. Exiting."
  exit 1
  ;;
esac

info "Starting the Kuadrant setup process... 🚀"

info "Checking prerequisites and dependencies... 🛠️"
check_dependencies

echo "Do you want to set up a DNS provider for use with Kuadrant's DNSPolicy API? (y/n)"
read -r SETUP_PROVIDER </dev/tty

case $SETUP_PROVIDER in
[Yy]*)
  requiredENV
  ;;
[Nn]*)
  echo "DNS provider setup skipped."
  ;;
*)
  error "Invalid input. Please respond with 'y' or 'n'. Exiting."
  exit 1
  ;;
esac

delete_networks() {
  for ((i = 1; i <= KUADRANT_CLUSTER_COUNT; i++)); do
    local network_name="kind-${i}"
    if ${CONTAINER_RUNTIME_BIN} network exists "${network_name}"; then
      echo "Deleting network ${network_name}"
      ${CONTAINER_RUNTIME_BIN} network rm "${network_name}" || true
    fi
  done
}

create_clusters() {
  local KUADRANT_CLUSTER_COUNT=$1
  local CONTAINER_RUNTIME=$(containerRuntime)
  info "Starting the setup process for $KUADRANT_CLUSTER_COUNT clusters... 🚀"

  for ((i = 1; i <= KUADRANT_CLUSTER_COUNT; i++)); do

    local cluster_name="kuadrant-local-${i}"
    local network_name="kind-${i}"
    
    # Start from a base offset of 90 to avoid clashing with existing 10.89.0.0/24 network
    local base=$((89 + i))
    local pod_subnet="10.${base}.0.0/16"
    local service_subnet="10.$((base + 100)).0.0/16"

    # Create Docker network
    if ! ${CONTAINER_RUNTIME_BIN} network create "${network_name}" --subnet "${pod_subnet}"; then
      error "Failed to create Docker network ${network_name}"
    fi

    # Create Kind cluster with specific configurations
    cat <<EOF | ${KIND_BIN} create cluster --name ${cluster_name} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  podSubnet: "${pod_subnet}"
  serviceSubnet: "${service_subnet}"
nodes:
- role: control-plane
  image: kindest/node:v1.29.2
EOF

    kubectl config use-context kind-${cluster_name}
    success "Kubernetes cluster ${cluster_name} created successfully."

    # Creating Kubernetes namespace
    info "Creating the necessary Kubernetes namespaces... 📦"
    kubectl create namespace kuadrant-system
    success "Kubernetes namespaces created successfully."

    # Installing Gateway API
    info "Installing Gateway API... 🌉"
    kubectl apply -k "${KUADRANT_GATEWAY_API_KUSTOMIZATION}"
    success "Gateway API installed successfully."

    # Installing Istio
    info "Installing Istio as a Gateway API provider... 🛫"
    kubectl apply -k "${KUADRANT_ISTIO_KUSTOMIZATION}"
    kubectl -n istio-system wait --for=condition=Available deployment istio-operator --timeout=300s
    success "Istio installed successfully."

    # Installing cert-manager
    info "Installing cert-manager... 🛡️"
    kubectl apply -k "${KUADRANT_CERT_MANAGER_KUSTOMIZATION}"
    kubectl -n cert-manager wait --for=condition=Available deployments --all --timeout=300s
    success "cert-manager installed successfully."

    # Installing MetalLB
    info "Installing MetalLB... 🏗️"
    kubectl apply -k "${KUADRANT_METALLB_KUSTOMIZATION}"
    kubectl -n metallb-system wait --for=condition=Available deployments controller --timeout=300s
    info "Generating IP address pool for MetalLB..."
    generate_ip_address_pool "${network_name}" | kubectl apply -n metallb-system -f -
    success "MetalLB installed and IP address pool generated successfully."

    # Installing Kuadrant
    info "Installing Kuadrant in ${cluster_name}..."
    kubectl apply -k "${KUADRANT_DEPLOY_KUSTOMIZATION}" --server-side --validate=false
    success "Kuadrant installed successfully."

    info "Kuadrant installation applied, configuring ManagedZone if DNS provider is set..."
    if [ ! -z "$DNS_PROVIDER" ]; then
      postSetup ${cluster_name} ${KUADRANT_NAMESPACE}
      setupClusterIssuer ${cluster_name}
    fi

    # Deploying Kuadrant sample configuration
    info "Deploying Kuadrant sample configuration..."
    kubectl -n kuadrant-system apply -f "${KUADRANT_REPO_RAW}/config/samples/kuadrant_v1beta1_kuadrant.yaml"
    success "Kuadrant sample configuration deployed."
  done

  info "✨🌟 Setup complete! All ${KUADRANT_CLUSTER_COUNT} clusters are ready. 🌟✨"
}

info "Deleting existing Kubernetes clusters if present... 🗑️"
for ((i = 1; i <= KUADRANT_CLUSTER_COUNT; i++)); do
  ${KIND_BIN} delete cluster --name "kuadrant-local-${i}"
done
success "Existing clusters (if present) deleted successfully."
delete_networks

create_clusters $KUADRANT_CLUSTER_COUNT

info "✨🌟 Setup complete! All ${KUADRANT_CLUSTER_COUNT} clusters are ready. 🌟✨"

info "Here's what has been configured:"
info "  - Kubernetes cluster with name '${KUADRANT_CLUSTER_NAME}'"
info "  - a Kuadrant namespace 'kuadrant-system'"
info "  - Gateway API"
info "  - Istio installed as a Gateway API provider"
info "  - cert-manager"
info "  - MetalLB with configured IP address pool"
info "  - Kuadrant components and a sample configuration"
if [ ! -z "$DNS_PROVIDER" ]; then
  info "  - DNS provider set to '${DNS_PROVIDER}'"
fi

info "Next steps:"
info "  - Explore your new Kuadrant environment using 'kubectl get all -n kuadrant-system'."
info "  - Head over to the Kuadrant quick start guide for further instructions on how to use Kuadrant with this environment:"
info "    🔗 https://docs.kuadrant.io/kuadrant-operator/doc/user-guides/secure-protect-connect/"

echo ""
info "Thank you for using Kuadrant! If you have any questions or feedback, please reach out to our community."
info "🔗 https://github.com/Kuadrant/"

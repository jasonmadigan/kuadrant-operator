#!/bin/bash

# Generates a MetalLB IpAddressPool for the given docker network.
# https://metallb.org/configuration/#defining-the-ips-to-assign-to-the-load-balancer-services
#
# Example:
# ./utils/docker-network-ipaddresspool.sh kind | kubectl apply -n metallb-system -f -

trap 'echo "Error on line $LINENO: $BASH_COMMAND"' ERR


set -euo pipefail

networkName=$1
YQ="${2:-yq}"

containerRuntime() {
  local container_runtime=""

  if command -v docker &> /dev/null; then
    container_runtime="docker"
  elif command -v podman &> /dev/null; then
    container_runtime="podman"
  else
    echo "Neither Docker nor Podman is installed. Exiting..."
    exit 1
  fi
  echo "$container_runtime"
}

export CONTAINER_RUNTIME_BIN=$(containerRuntime)

## Parse kind network subnet
## Take only IPv4 subnets, exclude IPv6
SUBNET=$($CONTAINER_RUNTIME_BIN network inspect $networkName --format '{{json .IPAM.Config }}' | \
    ${YQ} '.[] | select( .Subnet | test("^((25[0-5]|(2[0-4]|1\d|[1-9]|)\d)\.?\b){4}/\d+$")) | .Subnet')
if [[ -z "$SUBNET" ]]; then
   echo "Error: parsing IPv4 network address for '$networkName' docker network"
   exit 1
fi

# shellcheck disable=SC2206
subnetParts=(${SUBNET//./ })
cidr="${subnetParts[0]}.${subnetParts[1]}.200.0/24"

cat <<EOF | ADDRESS=$cidr ${YQ} '(select(.kind == "IPAddressPool") | .spec.addresses[0]) = env(ADDRESS)'
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kuadrant-local
spec:
  addresses: [] # set by make target
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: empty
EOF

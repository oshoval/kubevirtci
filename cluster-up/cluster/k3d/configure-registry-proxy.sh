# source: https://github.com/rpardini/docker-registry-proxy#kind-cluster
#
# This script execute docker-registry-proxy cluster nodes
# setup script on each cluster node.
# Basically what the setup script does is loading the proxy certificate
# and set HTTP_PROXY and NO_PROXY env vars to enable direct communication
# between cluster components (e.g: pods, nodes and services).
#
# Args:
# PROXY_HOSTNAME - docker-registry-proxy endpoint hostname.
# CLUSTER_NAME - K3d cluster name.
#
# Usage example:
# CLUSTER_NAME="sriov" PROXY_HOSTNAME="proxy.ci.com" \
#   ./configure-registry-proxy.sh
#

#! /bin/bash

set -ex

CRI_BIN=${CRI_BIN:-docker}

PROXY_HOSTNAME="${PROXY_HOSTNAME:-docker-registry-proxy}"
CLUSTER_NAME="${CLUSTER_NAME:-sriov}"

# TODO kubectl

SETUP_URL="http://${PROXY_HOSTNAME}:3128/setup/systemd"
pids=""
for node in $(kubectl get nodes -o=custom-columns=NAME:.metadata.name --no-headers); do
   echo $node # TODO
   #$CRI_BIN exec "$node" sh -c "\
   #   wget -O - -q $SETUP_URL | \
   #   sed s/docker\.service/containerd\.service/g | \
   #   sed '/Environment/ s/$/ \"NO_PROXY=127.0.0.0\/8,10.0.0.0\/8,172.16.0.0\/12,192.168.0.0\/16\"/' | \
   #   /bin/sh" &
   #pids="$pids $!"
done
#wait $pids


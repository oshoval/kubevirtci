#!/usr/bin/env bash

set -e

export CLUSTER_NAME="sriov"
export HOST_PORT=5000

DEPLOY_SRIOV=${DEPLOY_SRIOV:-true}

function print_available_nics() {
    echo 'STEP: Available NICs'
    # print hardware info for easier debugging based on logs
    ${CRI_BIN} run --rm --cap-add=SYS_RAWIO quay.io/phoracek/lspci@sha256:0f3cacf7098202ef284308c64e3fc0ba441871a846022bb87d65ff130c79adb1 sh -c "lspci | egrep -i 'network|ethernet'"
    echo
}

function print_agents_sriov_status() {
    nodes=$(_get_agent_nodes)
    echo "STEP: Print agents SR-IOV status"
    for node in $nodes; do
        echo "Node: $node"
        echo "VFs:"
        ${CRI_BIN} exec $node /bin/sh -c "ls -l /sys/class/net/*/device/virtfn*"
        echo "PFs PCI Addresses:"
        ${CRI_BIN} exec $node /bin/sh -c "grep PCI_SLOT_NAME /sys/class/net/*/device/uevent"
    done
    echo
}

function configure_registry_proxy() {
    # [ "$CI" != "true" ] && return

    echo "Configuring cluster nodes to work with CI mirror-proxy..."

    local -r ci_proxy_hostname="docker-mirror-proxy.kubevirt-prow.svc"
    local -r configure_registry_proxy_script="${KUBEVIRTCI_PATH}/cluster/k3d/configure-registry-proxy.sh"

    PROXY_HOSTNAME="$ci_proxy_hostname" $configure_registry_proxy_script
}

function up() {
    [ $DEPLOY_SRIOV == true ] && print_available_nics
    k3d_up
    configure_registry_proxy

    if [ $DEPLOY_SRIOV == true ]; then
        ${KUBEVIRTCI_PATH}/cluster/$KUBEVIRT_PROVIDER/config_sriov_cluster.sh
        print_agents_sriov_status
    fi

    version=$(_kubectl get node k3d-sriov-server-0 -o=custom-columns=VERSION:.status.nodeInfo.kubeletVersion --no-headers)
    echo "$KUBEVIRT_PROVIDER cluster '$CLUSTER_NAME' is ready ($version)"
}

source ${KUBEVIRTCI_PATH}/cluster/k3d/common.sh

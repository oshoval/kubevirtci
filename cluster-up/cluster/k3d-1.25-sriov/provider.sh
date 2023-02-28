#!/usr/bin/env bash

set -e

export CLUSTER_NAME="sriov"
export HOST_PORT=5000

function print_sriov_data() {
    nodes=$(_get_agent_nodes)
    echo "STEP: Print SR-IOV data"
    for node in $nodes; do
        echo "Node: $node"
        echo "VFs:"
        ${CRI_BIN} exec $node /bin/sh -c "ls -l /sys/class/net/*/device/virtfn*"
        echo "PFs PCI Addresses:"
        ${CRI_BIN} exec $node /bin/sh -c "grep PCI_SLOT_NAME /sys/class/net/*/device/uevent"
    done
    echo
}

function print_sriov_info() {
    echo 'STEP: Available NICs'
    # print hardware info for easier debugging based on logs
    ${CRI_BIN} run --rm --cap-add=SYS_RAWIO quay.io/phoracek/lspci@sha256:0f3cacf7098202ef284308c64e3fc0ba441871a846022bb87d65ff130c79adb1 sh -c "lspci | egrep -i 'network|ethernet'"
    echo
}

function up() {
    print_sriov_info
    k3d_up

    ${KUBEVIRTCI_PATH}/cluster/$KUBEVIRT_PROVIDER/config_sriov_cluster.sh

    print_sriov_data
    version=$(_kubectl get node k3d-sriov-server-0 -o=custom-columns=VERSION:.status.nodeInfo.kubeletVersion --no-headers)
    echo "$KUBEVIRT_PROVIDER cluster '$CLUSTER_NAME' is ready ($version)"
}

source ${KUBEVIRTCI_PATH}/cluster/k3d/common.sh

#!/usr/bin/env bash

set -e

export CLUSTER_NAME="sriov"
export K8S_VERSION="${K8S_VERSION:-1.17.0}"

function set_kind_params() {
    local k8s_version=$1
    
    if [ "$k8s_version" == "1.17.0" ]; then
        export KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.17.0}"
        export KIND_VERSION="${KIND_VERSION:-0.7.0}"
        export KUBECTL_PATH="${KUBECTL_PATH:-/kind/bin/kubectl}"
    elif [ "$k8s_version" == "1.19.1" ]; then
        export KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.19.1@sha256:98cf5288864662e37115e362b23e4369c8c4a408f99cbc06e58ac30ddc721600}"
        export KIND_VERSION="${KIND_VERSION:-0.9.0}"
        export KUBECTL_PATH="${KUBECTL_PATH:-/usr/bin/kubectl}"
    else 
        echo "Unsupported k8s version $k8s_version"
        exit 1
    fi
}

function up() {
    if [[ "$KUBEVIRT_NUM_NODES" -ne 2 ]]; then
        echo 'SR-IOV cluster can be only started with 2 nodes'
        exit 1
    fi

    # print hardware info for easier debugging based on logs
    echo 'Available NICs'
    docker run --rm --cap-add=SYS_RAWIO quay.io/phoracek/lspci@sha256:0f3cacf7098202ef284308c64e3fc0ba441871a846022bb87d65ff130c79adb1 sh -c "lspci | egrep -i 'network|ethernet'"
    echo ""

    cp $KIND_MANIFESTS_DIR/kind.yaml ${KUBEVIRTCI_CONFIG_PATH}/$KUBEVIRT_PROVIDER/kind.yaml

    kind_up

    # remove the rancher.io kind default storageClass
    _kubectl delete sc standard

    ${KUBEVIRTCI_PATH}/cluster/$KUBEVIRT_PROVIDER/config_sriov.sh
}

set_kind_params "$K8S_VERSION"

source ${KUBEVIRTCI_PATH}/cluster/kind/common.sh

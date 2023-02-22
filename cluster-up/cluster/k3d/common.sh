#!/usr/bin/env bash

set -e

function detect_cri() {
    if podman ps >/dev/null 2>&1; then echo podman; elif docker ps >/dev/null 2>&1; then echo docker; fi
}

export CRI_BIN=${CRI_BIN:-$(detect_cri)}

NODE_CMD="${CRI_BIN} exec -it -d "
export KUBEVIRTCI_PATH
export KUBEVIRTCI_CONFIG_PATH

REGISTRY_NAME=registry
BASE_PATH=${KUBEVIRTCI_CONFIG_PATH:-$PWD}

KUBERNETES_SERVICE_HOST=127.0.0.1
KUBERNETES_SERVICE_PORT=6443

KUBEVIRT_NUM_SERVERS=${KUBEVIRT_NUM_SERVERS:-3}

# ADAPT
function _wait_containers_ready {
    echo "STEP: Waiting for all containers to become ready ..."
    _kubectl wait --for=condition=Ready pod --all -n kube-system --timeout 5m
}

function _ssh_into_node() {
    # examples:
    # ./cluster-up/ssh.sh k3d-sriov-server-0 ls
    # ./cluster-up/ssh.sh k3d-sriov-server-0 /bin/sh
    ${CRI_BIN} exec -it "$@"
}

function _install_cnis {
    echo "STEP: install cnis"
    _install_cni_plugins
}

function _install_cni_plugins {
    # check CPU arch
    PLATFORM=$(uname -m)
    case ${PLATFORM} in
    x86_64* | i?86_64* | amd64*)
        ARCH="amd64"
        ;;
    ppc64le)
        ARCH="ppc64le"
        ;;
    aarch64* | arm64*)
        ARCH="arm64"
        ;;
    *)
        echo "ERROR: invalid Arch, only support x86_64, ppc64le, aarch64"
        exit 1
        ;;
    esac

    local CNI_VERSION="v0.8.5"
    local CNI_ARCHIVE="cni-plugins-linux-${ARCH}-$CNI_VERSION.tgz"
    local CNI_URL="https://github.com/containernetworking/plugins/releases/download/$CNI_VERSION/$CNI_ARCHIVE"
    if [ ! -f ${KUBEVIRTCI_CONFIG_PATH}/$KUBEVIRT_PROVIDER/$CNI_ARCHIVE ]; then
        echo "STEP: Downloading $CNI_ARCHIVE"
        curl -sSL -o ${KUBEVIRTCI_CONFIG_PATH}/$KUBEVIRT_PROVIDER/$CNI_ARCHIVE $CNI_URL
    fi

    for node in $(_get_nodes | awk '{print $1}'); do
        ${CRI_BIN} cp "${KUBEVIRTCI_CONFIG_PATH}/$KUBEVIRT_PROVIDER/$CNI_ARCHIVE" $node:/
        ${CRI_BIN} exec $node /bin/sh -c "mkdir -p /opt/cni/bin && tar -xvzf $CNI_ARCHIVE -C /opt/cni/bin" > /dev/null
    done
}

# ADAPT
function _prepare_config() {
    echo "STEP: prepare config"
    cat >$BASE_PATH/$KUBEVIRT_PROVIDER/config-provider-$KUBEVIRT_PROVIDER.sh <<EOF
master_ip=$KUBERNETES_SERVICE_HOST
kubeconfig=${BASE_PATH}/$KUBEVIRT_PROVIDER/.kubeconfig
kubectl=kubectl
docker_prefix=127.0.0.1:${HOST_PORT}/kubevirt
manifest_docker_prefix=$REGISTRY_NAME:$HOST_PORT/kubevirt
EOF
}

function _get_nodes() {
    _kubectl get nodes --no-headers
}

function _get_pods() {
    _kubectl get pods -A --no-headers
}

function _prepare_nodes {
    echo "STEP: prepare nodes"
    for node in $(_get_nodes | awk '{print $1}'); do
        ${CRI_BIN} exec $node /bin/sh -c "mount --make-rshared /"
    done
}

function setup_k3d() {
    TAG=v5.4.7
    curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=$TAG bash
}

function _print_kubeconfig() {
    echo "STEP: print kubeconfig"
    k3d kubeconfig print $CLUSTER_NAME > ${BASE_PATH}/$KUBEVIRT_PROVIDER/.kubeconfig
}

function k3d_up() {
    setup_k3d
    
    #k3d registry create --default-network bridge $REGISTRY_NAME --port $HOST_PORT
    k3d cluster create $CLUSTER_NAME --registry-create $REGISTRY_NAME:$KUBERNETES_SERVICE_HOST:$HOST_PORT --api-port $KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT \
                       --servers=$KUBEVIRT_NUM_SERVERS #--registry-use k3d-$REGISTRY_NAME:$HOST_PORT

    _print_kubeconfig
    _prepare_nodes
    _install_cnis
    _prepare_config
}

function _kubectl() {
    export KUBECONFIG=${BASE_PATH}/$KUBEVIRT_PROVIDER/.kubeconfig
    kubectl --kubeconfig=$KUBECONFIG "$@"
}

function down() {
    k3d cluster delete $CLUSTER_NAME
    docker rm --force $REGISTRY_NAME
}

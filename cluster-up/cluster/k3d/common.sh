#!/usr/bin/env bash

set -e

function detect_cri() {
    if podman ps >/dev/null 2>&1; then echo podman; elif docker ps >/dev/null 2>&1; then echo docker; fi
}

export CRI_BIN=${CRI_BIN:-$(detect_cri)}
KUBEVIRT_NUM_SERVERS=${KUBEVIRT_NUM_SERVERS:-1}
KUBEVIRT_NUM_AGENTS=${KUBEVIRT_NUM_AGENTS:-2}

export KUBEVIRTCI_PATH
export KUBEVIRTCI_CONFIG_PATH
BASE_PATH=${KUBEVIRTCI_CONFIG_PATH:-$PWD}

REGISTRY_NAME=registry
REGISTRY_HOST=127.0.0.1
KUBERNETES_SERVICE_HOST=127.0.0.1
KUBERNETES_SERVICE_PORT=6443

function _ssh_into_node() {
    # examples:
    # ./cluster-up/ssh.sh k3d-sriov-server-0 ls
    # ./cluster-up/ssh.sh k3d-sriov-server-0 /bin/sh
    ${CRI_BIN} exec -it "$@"
}

function _install_cnis {
    echo "STEP: Install cnis"
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

function _prepare_config() {
    echo "STEP: Prepare config"
    cat >$BASE_PATH/$KUBEVIRT_PROVIDER/config-provider-$KUBEVIRT_PROVIDER.sh <<EOF
master_ip=${KUBERNETES_SERVICE_HOST}
kubeconfig=${BASE_PATH}/$KUBEVIRT_PROVIDER/.kubeconfig
kubectl=kubectl
docker_prefix=${REGISTRY_HOST}:${HOST_PORT}/kubevirt
manifest_docker_prefix=${REGISTRY_NAME}:${HOST_PORT}/kubevirt
EOF
}

function _get_nodes() {
    _kubectl get nodes --no-headers
}

function _get_pods() {
    _kubectl get pods -A --no-headers
}

function _prepare_nodes {
    echo "STEP: Prepare nodes"
    for node in $(_get_nodes | awk '{print $1}'); do
        ${CRI_BIN} exec $node /bin/sh -c "mount --make-rshared /"
    done
}

function setup_k3d() {
    TAG=v5.4.7
    curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | TAG=$TAG bash
}

function _print_kubeconfig() {
    echo "STEP: Print kubeconfig"
    k3d kubeconfig print $CLUSTER_NAME > ${BASE_PATH}/$KUBEVIRT_PROVIDER/.kubeconfig
}

function k3d_up() {
    setup_k3d


    id1=${BASE_PATH}/$KUBEVIRT_PROVIDER/machine-id-1
    id2=${BASE_PATH}/$KUBEVIRT_PROVIDER/machine-id-2
    id3=${BASE_PATH}/$KUBEVIRT_PROVIDER/machine-id-3
    echo 11111111111111111111111111111111 > ${id1}
    echo 22222222222222222222222222222222 > ${id2}
    echo 33333333333333333333333333333333 > ${id3}

    k3d cluster create $CLUSTER_NAME --registry-create $REGISTRY_NAME:$REGISTRY_HOST:$HOST_PORT \
                       --api-port $KUBERNETES_SERVICE_HOST:$KUBERNETES_SERVICE_PORT \
                       --servers=$KUBEVIRT_NUM_SERVERS \
                       --agents=$KUBEVIRT_NUM_AGENTS \
                       --k3s-arg "--disable=traefik@server:0" \
                       --no-lb \
                       --k3s-arg "--flannel-backend=none@server:*" \
                       --k3s-arg "--kubelet-arg=feature-gates=CPUManager=true@server:0" \
                       --k3s-arg "--kubelet-arg=cpu-manager-policy=static@server:0" \
                       --k3s-arg "--kubelet-arg=kube-reserved=cpu=500m@server:0" \
                       --k3s-arg "--kubelet-arg=system-reserved=cpu=500m@server:0" \
                       --volume "$(pwd)/cluster-up/cluster/k3d/calico.yaml:/var/lib/rancher/k3s/server/manifests/calico.yaml@server:0" \
                       -v /dev/vfio:/dev/vfio@agent:* \
                       -v /lib/modules:/lib/modules@agent:* \
                       -v ${id1}:/etc/machine-id@server:0 \
                       -v ${id2}:/etc/machine-id@agent:0 \
                       -v ${id3}:/etc/machine-id@agent:1

    _print_kubeconfig
    _prepare_nodes
    _install_cnis
    _prepare_config

    kubectl label node k3d-sriov-agent-0 node-role.kubernetes.io/worker=worker
    kubectl label node k3d-sriov-agent-1 node-role.kubernetes.io/worker=worker
}

function _kubectl() {
    export KUBECONFIG=${BASE_PATH}/$KUBEVIRT_PROVIDER/.kubeconfig
    kubectl --kubeconfig=$KUBECONFIG "$@"
}

function down() {
    k3d cluster delete $CLUSTER_NAME
    ${CRI_BIN} rm --force $REGISTRY_NAME
}

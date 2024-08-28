package nodes

import (
	_ "embed"
	"fmt"

	"kubevirt.io/kubevirtci/cluster-provision/gocli/pkg/libssh"
)

//go:embed conf/00-cgroupv2.conf
var cgroupv2 []byte

type nodesProvisioner struct {
	sshClient   libssh.Client
	singleStack bool
}

func NewNodesProvisioner(sc libssh.Client, singleStack bool) *nodesProvisioner {
	return &nodesProvisioner{
		sshClient:   sc,
		singleStack: singleStack,
	}
}

func (n *nodesProvisioner) Exec() error {
	var (
		nodeIP         = ""
		controlPlaneIP = "192.168.66.101"
	)

	if n.singleStack {
		controlPlaneIP = "[fd00::101]"
		nodeIP = "--node-ip=::"
	}

	cmds := []string{
		"source /var/lib/kubevirtci/shared_vars.sh",
		`timeout=30; interval=5; while ! hostnamectl | grep Transient; do echo "Waiting for dhclient to set the hostname from dnsmasq"; sleep $interval; timeout=$((timeout - interval)); [ $timeout -le 0 ] && exit 1; done`,
		`echo "KUBELET_EXTRA_ARGS=--cgroup-driver=systemd --runtime-cgroups=/systemd/system.slice --kubelet-cgroups=/systemd/system.slice --fail-swap-on=false ` + nodeIP + ` --feature-gates=CPUManager=true,NodeSwap=true --cpu-manager-policy=static --kube-reserved=cpu=250m --system-reserved=cpu=250m" | tee /etc/sysconfig/kubelet > /dev/null`,
		`[ -f /sys/fs/cgroup/cgroup.controllers ] && mkdir -p /etc/crio/crio.conf.d && echo '` + string(cgroupv2) + `' |  tee /etc/crio/crio.conf.d/00-cgroupv2.conf > /dev/null &&  sed -i 's/--cgroup-driver=systemd/--cgroup-driver=cgroupfs/' /etc/sysconfig/kubelet && systemctl stop kubelet && systemctl restart crio`,
		"while [[ $(systemctl status crio | grep -c active) -eq 0 ]]; do sleep 2; done",
		"systemctl daemon-reload &&  service kubelet restart",
		"swapoff -a",
		"until ip address show dev eth0 | grep global | grep inet6; do sleep 1; done",
		"kubeadm join --token abcdef.1234567890123456 " + controlPlaneIP + ":6443 --ignore-preflight-errors=all --discovery-token-unsafe-skip-ca-verification=true",
		"mkdir -p /var/lib/rook",
		"chcon -t container_file_t /var/lib/rook",
	}

	for _, cmd := range cmds {
		err := n.sshClient.Command(cmd)
		if err != nil {
			return fmt.Errorf("error executing %s: %s", cmd, err)
		}
	}
	return nil
}
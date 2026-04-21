# Fedora Test-Tooling Cloud-Config Tweaks

- **Access and users**
  - Create users and set non-expiring passwords for `fedora` and `cloud-user`.

- **Kernel/module readiness**
  - Auto-load SR-IOV-related Mellanox/Intel NIC modules at boot.

- **Cloud-init and guest-agent ordering**
  - Add wait service so guest agent starts after cloud-init.
  - Enable both services.

- **Console stability for tests**
  - Disable bracketed-paste output noise.
  - Disable systemd shell integration escapes.

- **Faster datasource detection**
  - Limit datasource list to `NoCloud`, `ConfigDrive`, `None`.

- **Network and resolver behavior**
  - Restore classic interface naming (`eth0`).
  - Set NSS hosts order to `files dns myhostname`.

- **Packages and netcat behavior**
  - Install VM/e2e tooling (guest agent, stress, perf/network/debug utilities).
  - Keep `nc` as OpenBSD netcat for cloud-init; also install `ncat` (`nmap-ncat`) since KubeVirt e2e uses both.

- **Image/test compatibility adjustments**
  - Clear static + transient hostname.
  - Remove `users_groups` so cloud-init does not remove the configured users.
  - Set SELinux permissive.
  - Remove `pam_nologin` from sshd PAM.

- **Architecture-specific boot safety**
  - On `s390x`, regenerate initramfs and boot artifacts.

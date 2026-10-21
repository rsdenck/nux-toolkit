<div align="center">
  <h1 style="color: #00ff00; background-color: #000000; padding: 15px; border: 2px solid #00ff00; border-radius: 10px; display: inline-block;">NUX Toolkit – Professional Automation Suite</h1>
  <p style="font-size: 1.2em; color: #00ff00;">Robust, idempotent, and multi-distribution Linux tools that NUX can discover and use — each one shipped as a polyglot pair (Go + Bash).</p>

  [![License: MIT](https://img.shields.io/badge/License-MIT-00ff00.svg?style=for-the-badge&logo=opensourceinitiative&logoColor=black&labelColor=black)](https://opensource.org/licenses/MIT)
  [![Platform](https://img.shields.io/badge/Platform-Linux-00ff00.svg?style=for-the-badge&logo=linux&logoColor=black&labelColor=black)](https://www.linux.org/)
  [![Go](https://img.shields.io/badge/Go-00ff00.svg?style=for-the-badge&logo=go&logoColor=black&labelColor=black)](https://go.dev/)
  [![Shell](https://img.shields.io/badge/Shell-Bash-00ff00.svg?style=for-the-badge&logo=gnu-bash&logoColor=black&labelColor=black)](https://www.gnu.org/software/bash/)
</div>

---

## 🟢 **Overview**
The **NUX Toolkit** is a high-performance library of automation tools designed to streamline server management, security auditing, and environment provisioning. It carries the GETDEV script library forward in a new home.

Every tool is delivered as a **polyglot pair**:
- `<tool>.go` — the complete implementation, written in Go.
- `<tool>.sh` — the **same file, byte for byte**: a small bash/sh header compiles and runs the embedded Go implementation.

Built under the **Clean Room Quality** standard, every tool ensures:
- **Idempotency**: Safe to run multiple times without unintended side effects.
- **Multi-Distro Support**: Compatible with Ubuntu, Debian, CentOS/RHEL/Rocky/Alma, Fedora, and Arch.
- **Single Source of Truth**: No duplicated logic — the bash entry point never reimplements anything.
- **Security First**: Professional hardening and auditing capabilities.

---

## 🟢 **Category Structure**
Tools are logically categorized for rapid deployment.

| Category | Description | Representative Tool |
| :--- | :--- | :--- |
| **ai** | Artificial Intelligence tools and setups | `ai/ollama.sh` |
| **cloud** | Automation for Cloud Providers (GCP, AWS, Azure) | `cloud/gcloud.sh` |
| **ct** | Container Stack (Docker, Podman) | `ct/docker.sh` |
| **db** | Database tuning and configuration | `db/README.md` |
| **dev** | Development environments and languages | `dev/node.sh` |
| **hard** | Hardening tools (App, DB, SSH) | `hard/hard_ssh.sh` |
| **k8s** | Kubernetes & Ceph forensic audits | `k8s/analyzer.sh` |
| **net** | Network analysis and tools (TUI) | `net/net_analyzer.sh` |
| **os** | OS bootstrap, storage and SSH Manager (SSHM) | `os/sshm_install.sh` |
| **sec** | Cyber Security (Blue, Red, Purple, Black Teams) | `sec/warfire.sh` |
| **tools** | Miscellaneous utilities and CLI tools | `tools/README.md` |

---

## 🟢 **How to Use**
Every tool runs from Bash or straight from source with Go:

```bash
# Option A — Bash (requires the Go toolchain to compile the embedded implementation)
curl -fsSL https://raw.githubusercontent.com/rsdenck/nux-toolkit/main/dev/node.sh -o node.sh
bash node.sh

# Option B — Go
curl -fsSL https://raw.githubusercontent.com/rsdenck/nux-toolkit/main/dev/node.go -o node.go
go run node.go
```
> **Note:** Tools are distribution-aware and handle package management for the specific OS. The Bash entry point never installs anything by itself — it only compiles and executes the Go implementation.

---

## 🟢 **Toolkit Contents**

| Tool | Category | Description |
| :--- | :--- | :--- |
| `ai/ollama.sh` | ai | Configure Ollama globally for the VS Code Continue extension and pull the models. |
| `cloud/gcloud.sh` | cloud | Bootstrap the Google Cloud SDK (`gcloud` CLI). |
| `ct/docker.sh` | ct | Install Docker CE with buildx/compose plugins and apply daemon tuning. |
| `ct/podman.sh` | ct | Install Podman, Buildah and Skopeo. |
| `dev/node.sh` | dev | Node.js frontend environment: NVM, Node LTS, Yarn and PNPM. |
| `hard/hard_app.sh` | hard | OS tuning for middleware servers (Java/Node): sysctl + resource limits. |
| `hard/hard_db.sh` | hard | OS tuning for database servers: shared memory, dirty pages, THP. |
| `hard/hard_ssh.sh` | hard | SSH hardening (port 4022, no root/password login) + auditd. |
| `k8s/analyzer.sh` | k8s | Interactive Ceph/Rook cluster analyzer with health score (whiptail menu). |
| `k8s/audit_layer1_host_disk.sh` | k8s | Passive host/disk audit: PSI, read-only mounts, SMART, I/O throttling. |
| `k8s/audit_layer2_os_health.sh` | k8s | OS health audit: CPU, memory, kernel, cgroups, network. |
| `k8s/audit_layer3_k8s_containers.sh` | k8s | Kubernetes/containers audit: kubelet, pods, events, runtimes. |
| `k8s/audit_layer4_ceph_forensics.sh` | k8s | Read-only Ceph forensics: status, OSDs, PGs (no recovery/scrub impact). |
| `k8s/audit_layer5_bluestore_rocksdb.sh` | k8s | BlueStore/RocksDB indirect audit: corruption, WAL/DB saturation, OSD→disk map. |
| `k8s/ceph_osd.sh` | k8s | Quick OSD/PG resource-conflict report for pool 44. |
| `net/net_analyzer.sh` | net | Network TUI: interfaces, public IP geolocation and download speed test. |
| `os/ansible.sh` | os | Ansible LTS bootstrap on RHEL-like systems. |
| `os/apexsc.sh` | os | Apex Secure Connectivity: cloudflared + Tailscale. |
| `os/audit.sh` | os | Defensive audit: journal persistence, root logins, brute-force, suspicious ports. |
| `os/extend_disk.sh` | os | Guided disk/partition/filesystem extension (rescan, growpart, resize2fs/xfs_growfs). |
| `os/extend_lvm.sh` | os | Interactive LVM extension (PV → VG → LV → filesystem). |
| `os/extend_swap.sh` | os | Create, enable and persist a swapfile. |
| `os/javadeb.sh` | os | OpenJDK 11/17/21 on Debian-like systems, newest as default. |
| `os/javarhel.sh` | os | OpenJDK 11/17/21 on RHEL-like systems, newest as default. |
| `os/lunar.sh` | os | Wrapper for LUNAR (Lockdown UNix Auditing and Reporting). |
| `os/osdeb.sh` | os | Essential SysAdmin tools bootstrap for Debian-like systems (~50 packages). |
| `os/osrhel.sh` | os | Essential SysAdmin tools bootstrap for RHEL-like systems (~50 packages + EPEL). |
| `os/postgrhel.sh` | os | PostgreSQL 16 bootstrap (PGDG repo) with initdb and service. |
| `os/sshm.sh` | os | Create a user and harden SSH (port, root/password off, AllowUsers) + auditd. |
| `os/sshm_install.sh` | os | Installer for the `sshm` SSH manager. |
| `os/terraform.sh` | os | Terraform from the official HashiCorp repository (RHEL-like). |
| `os/tofudeb.sh` | os | OpenTofu on Debian-like systems. |
| `os/tofurhel.sh` | os | OpenTofu on RHEL-like systems. |
| `os/tshardeb.sh` | os | tshark (Wireshark CLI) on Debian-like systems. |
| `os/tsharhel.sh` | os | tshark (Wireshark CLI) on RHEL-like systems (EPEL + capabilities). |
| `sec/black.sh` | sec | Black Team reconnaissance: system discovery, exposed credentials, exfiltration paths. |
| `sec/blue.sh` | sec | Blue Team defense: SSH, firewall (UFW/firewalld/iptables) and sysctl hardening. |
| `sec/purple.sh` | sec | Purple Team orchestrator: runs the Red audit, then the Blue defense, then verifies. |
| `sec/red.sh` | sec | Red Team read-only audit: SUID/SGID, sensitive files, capabilities, sockets. |
| `sec/warfire.sh` | sec | Emergency incident response: rootkit detection, evidence collection, lockdown. |

---

## 🟢 **Skills & Tools**

### **Core Competencies**
![Bash](https://img.shields.io/badge/Bash-black?style=for-the-badge&logo=gnu-bash&logoColor=00ff00)
![Python](https://img.shields.io/badge/Python-black?style=for-the-badge&logo=python&logoColor=00ff00)
![PHP](https://img.shields.io/badge/PHP-black?style=for-the-badge&logo=php&logoColor=00ff00)
![Go](https://img.shields.io/badge/Go-black?style=for-the-badge&logo=go&logoColor=00ff00)
![Git](https://img.shields.io/badge/Git-black?style=for-the-badge&logo=git&logoColor=00ff00)

### **DevOps & Infrastructure**
![Docker](https://img.shields.io/badge/Docker-black?style=for-the-badge&logo=docker&logoColor=00ff00)
![Podman](https://img.shields.io/badge/Podman-black?style=for-the-badge&logo=podman&logoColor=00ff00)
![Ansible](https://img.shields.io/badge/Ansible-black?style=for-the-badge&logo=ansible&logoColor=00ff00)
![Terraform](https://img.shields.io/badge/Terraform-black?style=for-the-badge&logo=terraform&logoColor=00ff00)
![Ollama](https://img.shields.io/badge/Ollama-black?style=for-the-badge&logo=ollama&logoColor=00ff00)
![Zabbix](https://img.shields.io/badge/Zabbix-black?style=for-the-badge&logo=zabbix&logoColor=00ff00)
![VMware](https://img.shields.io/badge/VMware-black?style=for-the-badge&logo=vmware&logoColor=00ff00)
![Proxmox](https://img.shields.io/badge/Proxmox-black?style=for-the-badge&logo=proxmox&logoColor=00ff00)
![KVM](https://img.shields.io/badge/KVM-black?style=for-the-badge&logo=linux&logoColor=00ff00)
![Ceph](https://img.shields.io/badge/Ceph-black?style=for-the-badge&logo=ceph&logoColor=00ff00)
![Grafana](https://img.shields.io/badge/Grafana-black?style=for-the-badge&logo=grafana&logoColor=00ff00)
![APIs](https://img.shields.io/badge/APIs-black?style=for-the-badge&logo=fastapi&logoColor=00ff00)
![Kubernetes](https://img.shields.io/badge/Kubernetes-black?style=for-the-badge&logo=kubernetes&logoColor=00ff00)
![GCP](https://img.shields.io/badge/GCP-black?style=for-the-badge&logo=google-cloud&logoColor=00ff00)

### **Complementary Skills**
`HTML5` • `CSS3` • `JavaScript` • `MySQL` • `Figma` • `Handlebars`

---

## 🟢 **Activity Graph**
[![ranlens's github activity graph](https://github-readme-activity-graph.vercel.app/graph?username=rsdenck&theme=react-dark&bg_color=000000&color=00ff00&line=00ff00&point=00ff00&area=true&hide_border=true&custom_title=NUX%20Toolkit%20Activity%20Graph)](https://github.com/rsdenck/nux-toolkit)

---

## 🟢 **Contribution**
We welcome contributions that adhere to our **Clean Room Quality** guidelines.

### **Rules**
1.  **Strict LF-only**: Use Unix line endings.
2.  **UTF-8 Clean**: No BOM, no broken characters.
3.  **Idempotency**: Tools must be safe to re-run.
4.  **Polyglot Pairs**: Every tool ships as `<tool>.go` + `<tool>.sh` with byte-identical content; keep both in sync and `gofmt` clean.
5.  **ASCII Comments**: Keep technical documentation clean and compatible.

### **Workflow**
```bash
# 1. Branch out
git checkout -b feature/new-automation

# 2. Commit your changes
git add .
git commit -m "feat: add professional audit tool"

# 3. Push and PR
git push origin feature/new-automation
```

---

<div align="center">
  <p style="color: #00ff00; background-color: #000000; padding: 10px; border-top: 1px solid #00ff00;">
    <b>Developed by ranlens.denck</b><br>
    Copyright © 2026 NUX Toolkit | Clean Room Quality Guaranteed<br>
    <a href="https://opensource.org/licenses/MIT" style="color: #00ff00;">MIT License</a>
  </p>
</div>

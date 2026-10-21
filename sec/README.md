# Security Module (sec)

This directory contains professional security tools for auditing, system hardening, and team exercises. These are real functional scripts designed for security professionals and sysadmins.

## Scripts

- **[red.sh](red.sh)**: **Red Team Security Auditor**. Performs real offensive reconnaissance, hunts for SUID binaries, discovers sensitive files, and audits process permissions.
- **[blue.sh](blue.sh)**: **Blue Team Defense & Hardening**. Applies real security measures: SSH hardening, firewall configuration (UFW/Firewalld/Iptables), kernel sysctl hardening, and security monitoring checks.
- **[warfire.sh](warfire.sh)**: **ULTIMATE DEFENDER (First Line of Defense)**. Emergency script for compromised servers. Features the **AIX Engine** for threat detection, **Isolation Mode** to trap attackers, and automated forensic evidence collection.
- **[purple.sh](purple.sh)**: **Purple Team Orchestrator**. Coordinates an end-to-end security exercise: runs the Red Team audit, then the Blue Team defense, and finally verifies the security improvements.
- **[black.sh](black.sh)**: **Black Team Reconnaissance**. Focuses on deep system discovery, hunting for exposed credentials, and testing data exfiltration paths.

## Usage

All scripts should be run with appropriate permissions. Defense scripts (`blue.sh`) and deep audits (`red.sh`) **require root/sudo** for full functionality. Each tool ships as a polyglot pair (`red.go` + `red.sh`, byte-identical).

```bash
# To run a full Purple Team exercise (Attack -> Defense -> Verify)
sudo ./purple.sh

# To perform a standalone security audit
sudo ./red.sh

# To harden the system immediately
sudo ./blue.sh
```

## Standards

All scripts in this module follow the **GETDEV Clean Room Quality** standards:
- **UTF-8 without BOM**: Maximum compatibility across all editors.
- **LF line endings**: Native Linux format.
- **English Descriptions**: Professional technical standard.
- **Standardized Headers**: Clear project and script identification.
- **Color-coded Logging**: 
  - `[INFO]` (Green): Normal activity.
  - `[WARN]` (Yellow/Blue): Security warnings or audit findings.
  - `[CRIT/ERROR]` (Red): Critical vulnerabilities or script failures.

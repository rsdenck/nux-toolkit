/* 2>/dev/null
# ---------------------------------------------------------------------------
#  hard_app.sh — polyglot: este mesmo arquivo é hard_app.go (Go) e hard_app.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run hard_app.go [argumentos]
#    Bash ...: bash hard_app.sh  [argumentos]   (exige a toolchain Go)
#
# ------------------------------------------------------------------------------------
# Projeto: GETDEV
# Script: hard_app.sh
# Description: OS tuning and hardening for Middleware servers (Java, Node, etc)
# Author: ranlens.denck
# Category: hard
# Compatibility: Ubuntu, Debian, CentOS, Fedora, Arch
# ------------------------------------------------------------------------------------
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[hard_app.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[hard_app.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-hard_app"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/hard_app.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init hard_app >/dev/null 2>&1) || :

_bin="$_trab/hard_app"
if [ ! -x "$_bin" ] || [ "$_trab/hard_app.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando hard_app.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o hard_app.tmp hard_app.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o hard_app.tmp hard_app.go) ||
			_die "falha ao compilar $_trab/hard_app.go"
	}
	mv -f "$_trab/hard_app.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export HARD_APP_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (hard_app.go/hard_app.sh) ---'
*/
// hard_app.go — OS tuning and hardening for Middleware servers (Java, Node, etc).
//
// Projeto: GETDEV
// Script: hard_app.sh
// Author: ranlens.denck
// Category: hard
// Compatibility: Ubuntu, Debian, CentOS, Fedora, Arch
//
// Execução:
//
//	go run hard_app.go [argumentos]
//	bash hard_app.sh  [argumentos]
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	colorGreen  = "\033[0;32m"
	colorRed    = "\033[0;31m"
	colorYellow = "\033[1;33m"
	colorNC     = "\033[0m"
)

var (
	scriptName = "hard_app.sh"
	backupDir  string
	sysctlFile = "/etc/sysctl.d/99-getdev-app.conf"
	limitsFile = "/etc/security/limits.d/99-getdev-app.conf"
	distro     = "unknown"
)

func logInfo(msg string) {
	if isTTY() && noColor() == "" {
		fmt.Printf("%s[INFO]%s %s\n", colorGreen, colorNC, msg)
		return
	}
	fmt.Printf("[INFO] %s\n", msg)
}

func logWarn(msg string) {
	if isTTY() && noColor() == "" {
		fmt.Printf("%s[WARN]%s %s\n", colorYellow, colorNC, msg)
		return
	}
	fmt.Printf("[WARN] %s\n", msg)
}

func logErr(msg string) {
	if isTTY() && noColor() == "" {
		fmt.Fprintf(os.Stderr, "%s[ERROR]%s %s\n", colorRed, colorNC, msg)
		return
	}
	fmt.Fprintf(os.Stderr, "[ERROR] %s\n", msg)
}

func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func noColor() string {
	return os.Getenv("NO_COLOR")
}

func checkRoot() {
	if os.Geteuid() != 0 {
		logErr("This script must be run as root. Use sudo.")
		os.Exit(1)
	}
}

func detectDistro() {
	if _, err := os.Stat("/etc/os-release"); err == nil {
		f, err := os.Open("/etc/os-release")
		if err == nil {
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "ID=") {
					val := strings.TrimPrefix(line, "ID=")
					val = strings.Trim(val, `"`)
					distro = val
				}
				if strings.HasPrefix(line, "NAME=") {
					val := strings.TrimPrefix(line, "NAME=")
					val = strings.Trim(val, `"`)
					logInfo(fmt.Sprintf("Distribution detected: %s (%s)", val, distro))
					return
				}
			}
		}
	}
	logWarn("Distribution not clearly identified. Proceeding with caution.")
	distro = "unknown"
}

func mkBackupDir() {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		// best effort
	}
	logInfo(fmt.Sprintf("Backup directory created: %s", backupDir))
}

func readMemTotalKB() int {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, err := strconv.Atoi(fields[1])
				if err == nil {
					return v
				}
			}
		}
	}
	return 0
}

func getNproc() int {
	cmd := exec.Command("nproc", "--all")
	out, err := cmd.Output()
	if err != nil {
		// `CPUS=$(nproc --all)` sob set -euo pipefail: falha do comando
		// aborta o original com o próprio código de saída.
		code := 1
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() >= 0 {
			code = ee.ExitCode()
		} else {
			fmt.Fprintln(os.Stderr, "nproc: command not found")
			code = 127
		}
		os.Exit(code)
	}
	s := strings.TrimSpace(string(out))
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return 1
}

func calcRecommendations() (swappiness, fsFileMax, maxMapCount, somaxconn, netCoreRmemDefault, netCoreWmemDefault, netdevBacklog, portRangeStart, portRangeEnd, cachePressure, dirtyRatio, dirtyBackground int) {
	logInfo("Calculating recommendations based on hardware...")
	memTotalKB := readMemTotalKB()
	memTotalMB := memTotalKB / 1024
	_ = getNproc() // calculated but not used in original - removed from logic, just noted

	swappiness = 10
	fsFileMax = memTotalMB * 200
	if fsFileMax < 500000 {
		fsFileMax = 500000
	}
	maxMapCount = 524288
	somaxconn = 65535
	netCoreRmemDefault = 262144
	netCoreWmemDefault = 262144
	netdevBacklog = 500000
	portRangeStart = 10240
	portRangeEnd = 65535
	cachePressure = 75
	dirtyRatio = 15
	dirtyBackground = 5
	return
}

func generateSysctl(swappiness, fsFileMax, maxMapCount, somaxconn, netCoreRmemDefault, netCoreWmemDefault, netdevBacklog, portRangeStart, portRangeEnd, cachePressure, dirtyRatio, dirtyBackground int) {
	logInfo("Generating sysctl configurations...")
	content := fmt.Sprintf(`# GETDEV Hardening - Middleware
# Date: %s
vm.swappiness = %d
vm.vfs_cache_pressure = %d
vm.overcommit_memory = 1
vm.dirty_ratio = %d
vm.dirty_background_ratio = %d
fs.file-max = %d
vm.max_map_count = %d
net.core.somaxconn = %d
net.core.netdev_max_backlog = %d
net.core.rmem_default = %d
net.core.wmem_default = %d
net.core.rmem_max = %d
net.core.wmem_max = %d
net.ipv4.ip_local_port_range = %d %d
net.ipv4.tcp_max_syn_backlog = 65536
net.ipv4.tcp_fin_timeout = 10
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_sack = 1
net.ipv4.tcp_timestamps = 1
net.ipv4.tcp_window_scaling = 1
`, time.Now().Format(time.RFC1123), swappiness, cachePressure, dirtyRatio, dirtyBackground, fsFileMax, maxMapCount, somaxconn, netdevBacklog, netCoreRmemDefault, netCoreWmemDefault, netCoreRmemDefault*16, netCoreWmemDefault*16, portRangeStart, portRangeEnd)
	if err := os.WriteFile(sysctlFile+".tmp", []byte(content), 0644); err != nil {
		logErr(err.Error())
		os.Exit(1)
	}
}

func generateLimits() {
	logInfo("Generating limits configurations...")
	content := `# GETDEV Hardening - Middleware
* soft nofile  200000
* hard nofile  500000
* soft nproc   131072
* hard nproc   131072
* soft core 0
`
	if err := os.WriteFile(limitsFile+".tmp", []byte(content), 0644); err != nil {
		logErr(err.Error())
		os.Exit(1)
	}
}

func applyChanges() {
	checkRoot()
	detectDistro()
	mkBackupDir()
	swappiness, fsFileMax, maxMapCount, somaxconn, netCoreRmemDefault, netCoreWmemDefault, netdevBacklog, portRangeStart, portRangeEnd, cachePressure, dirtyRatio, dirtyBackground := calcRecommendations()

	if _, err := os.Stat(sysctlFile); err == nil {
		copyFile(sysctlFile, backupDir+"/sysctl.conf.bak")
	}
	generateSysctl(swappiness, fsFileMax, maxMapCount, somaxconn, netCoreRmemDefault, netCoreWmemDefault, netdevBacklog, portRangeStart, portRangeEnd, cachePressure, dirtyRatio, dirtyBackground)
	if err := os.Rename(sysctlFile+".tmp", sysctlFile); err != nil {
		logErr(err.Error())
		os.Exit(1)
	}
	cmd := exec.Command("sysctl", "--system")
	cmd.Stdout = nil // original: sysctl --system > /dev/null (só o stdout)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// set -euo pipefail: falha do sysctl aborta com o código do comando
		code := 1
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() >= 0 {
			code = ee.ExitCode()
		}
		os.Exit(code)
	}
	logInfo(fmt.Sprintf("Sysctl applied and persisted in %s", sysctlFile))

	if _, err := os.Stat(limitsFile); err == nil {
		copyFile(limitsFile, backupDir+"/limits.conf.bak")
	}
	generateLimits()
	if err := os.Rename(limitsFile+".tmp", limitsFile); err != nil {
		logErr(err.Error())
		os.Exit(1)
	}
	logInfo(fmt.Sprintf("Limits applied and persisted in %s", limitsFile))

	logInfo("Middleware hardening completed successfully!")
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		return
	}
	_ = os.WriteFile(dst, data, 0644)
}

func main() {
	backupDir = fmt.Sprintf("/var/backups/getdev/%s_%s", scriptName, time.Now().Format("20060102_150405"))
	// case "${1:-}" in --apply): só o $1 importa, extras são ignorados
	if len(os.Args) >= 2 && os.Args[1] == "--apply" {
		applyChanges()
		return
	}
	fmt.Print("Usage: hard_app.sh --apply\n")
	os.Exit(1)
}

// --- fim do arquivo polyglot (hard_app.go/hard_app.sh) ---

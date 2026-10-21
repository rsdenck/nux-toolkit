/* 2>/dev/null
# ---------------------------------------------------------------------------
#  hard_db.sh — polyglot: este mesmo arquivo é hard_db.go (Go) e hard_db.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run hard_db.go [argumentos]
#    Bash ...: bash hard_db.sh  [argumentos]   (exige a toolchain Go)
#
# ------------------------------------------------------------------------------------
# Projeto: GETDEV
# Script: hard_db.sh
# Description: OS tuning and hardening for Database servers (PostgreSQL, MySQL, Oracle)
# Author: ranlens.denck
# Category: hard
# Compatibility: Ubuntu, Debian, CentOS, Fedora, Arch
# ------------------------------------------------------------------------------------
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[hard_db.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[hard_db.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-hard_db"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/hard_db.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init hard_db >/dev/null 2>&1) || :

_bin="$_trab/hard_db"
if [ ! -x "$_bin" ] || [ "$_trab/hard_db.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando hard_db.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o hard_db.tmp hard_db.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o hard_db.tmp hard_db.go) ||
			_die "falha ao compilar $_trab/hard_db.go"
	}
	mv -f "$_trab/hard_db.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export HARD_DB_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (hard_db.go/hard_db.sh) ---'
*/
// hard_db.go — OS tuning and hardening for Database servers (PostgreSQL, MySQL, Oracle).
//
// Projeto: GETDEV
// Script: hard_db.sh
// Author: ranlens.denck
// Category: hard
// Compatibility: Ubuntu, Debian, CentOS, Fedora, Arch
//
// Execução:
//
//	go run hard_db.go [argumentos]
//	bash hard_db.sh  [argumentos]
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
	scriptName = "hard_db.sh"
	backupDir  string
	sysctlFile = "/etc/sysctl.d/99-getdev-db.conf"
	limitsFile = "/etc/security/limits.d/99-getdev-db.conf"
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
	logWarn("Distribution not clearly identified.")
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

func calcRecommendations() (fsFileMax, maxMapCount int, shmmax int64, somaxconn int) {
	logInfo("Calculating recommendations for Database...")
	memTotalKB := readMemTotalKB()
	memTotalMB := memTotalKB / 1024

	fsFileMax = memTotalMB * 100
	if fsFileMax < 200000 {
		fsFileMax = 200000
	}
	maxMapCount = 262144
	shmmax = int64(memTotalKB) / 4 * 1024
	somaxconn = 65535

	_ = shmmax / 4096
	return
}

func disableTHP() {
	if _, err := os.Stat("/sys/kernel/mm/transparent_hugepage"); err == nil {
		logInfo("Disabling Transparent HugePages (THP) for DB performance...")
		_ = os.WriteFile("/sys/kernel/mm/transparent_hugepage/enabled", []byte("never\n"), 0644)
		_ = os.WriteFile("/sys/kernel/mm/transparent_hugepage/defrag", []byte("never\n"), 0644)
	}
}

func applyChanges() {
	checkRoot()
	detectDistro()
	mkBackupDir()
	fsFileMax, maxMapCount, shmmax, somaxconn := calcRecommendations()

	content := fmt.Sprintf(`# GETDEV Hardening - Database
vm.swappiness = 1
vm.vfs_cache_pressure = 50
vm.overcommit_memory = 1
vm.max_map_count = %d
vm.dirty_expire_centisecs = 3000
vm.dirty_writeback_centisecs = 500
vm.dirty_ratio = 10
vm.dirty_background_ratio = 5
kernel.shmmax = %d
kernel.shmall = %d
fs.file-max = %d
net.core.somaxconn = %d
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15
`, maxMapCount, shmmax, shmmax/4096, fsFileMax, somaxconn)
	if err := os.WriteFile(sysctlFile+".tmp", []byte(content), 0644); err != nil {
		logErr(err.Error())
		os.Exit(1)
	}
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

	limitsContent := `# GETDEV Hardening - Database
* soft nofile  65536
* hard nofile  200000
* soft nproc   65536
* hard nproc   65536
* soft core 0
`
	if err := os.WriteFile(limitsFile+".tmp", []byte(limitsContent), 0644); err != nil {
		logErr(err.Error())
		os.Exit(1)
	}
	if err := os.Rename(limitsFile+".tmp", limitsFile); err != nil {
		logErr(err.Error())
		os.Exit(1)
	}

	disableTHP()
	logInfo("Database hardening completed!")
}

func main() {
	backupDir = fmt.Sprintf("/var/backups/getdev/%s_%s", scriptName, time.Now().Format("20060102_150405"))
	// case "${1:-}" in --apply): só o $1 importa, extras são ignorados
	if len(os.Args) >= 2 && os.Args[1] == "--apply" {
		applyChanges()
		return
	}
	fmt.Print("Usage: hard_db.sh --apply\n")
	os.Exit(1)
}

// --- fim do arquivo polyglot (hard_db.go/hard_db.sh) ---

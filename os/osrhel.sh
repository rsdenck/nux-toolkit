/* 2>/dev/null
# ---------------------------------------------------------------------------
#  osrhel.sh — polyglot: este mesmo arquivo é osrhel.go (Go) e osrhel.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run osrhel.go [argumentos]
#    Bash ...: bash osrhel.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      osrhel.sh
#  Description: Essential SysAdmin tools bootstrap for RHEL-like systems
#  Author:      GETDEV Team
#  Category:    OS / Bootstrap
#  Compatibility: RHEL, Rocky, Alma, Oracle Linux
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[osrhel.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[osrhel.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-osrhel"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/osrhel.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init osrhel >/dev/null 2>&1) || :

_bin="$_trab/osrhel"
if [ ! -x "$_bin" ] || [ "$_trab/osrhel.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando osrhel.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o osrhel.tmp osrhel.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o osrhel.tmp osrhel.go) ||
			_die "falha ao compilar $_trab/osrhel.go"
	}
	mv -f "$_trab/osrhel.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (osrhel.go/osrhel.sh) ---'
*/
// osrhel.go — bootstrap de ferramentas essenciais de sysadmin em RHEL-like.
//
// Porta fiel do script original osrhel.sh (GETDEV): exige root, espelha stdout
// e stderr em /var/log/osrhel-bootstrap.log (append) e no console, habilita os
// repositórios essenciais (EPEL + dnf-plugins-core), instala os pacotes
// essenciais, habilita firewalld/auditd/fail2ban, aplica o tuning básico de
// file descriptors e finaliza com "dnf autoremove -y". Nenhum texto de saída
// foi traduzido.
//
// Execução:
//
//	go run osrhel.go
//	bash osrhel.sh          (mesmo arquivo, polyglot Go + sh)
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ---------------------------------------------------------------------------
// Saída (mesmas cores e labels do original; cor só em TTY e sem NO_COLOR)
// ---------------------------------------------------------------------------

var corAtiva bool

const (
	cReset = "\033[0m"
	cVerde = "\033[0;32m"
	cVerm  = "\033[0;31m"
)

func pintar(code, s string) string {
	if !corAtiva {
		return s
	}
	return code + s + cReset
}

func prepararSaida() {
	if os.Getenv("NO_COLOR") != "" {
		return
	}
	if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		corAtiva = true
	}
}

func logInfo(msg string) {
	fmt.Fprintln(os.Stdout, pintar(cVerde, "[INFO]")+" "+msg)
}

func logError(msg string) {
	fmt.Fprintln(os.Stderr, pintar(cVerm, "[ERROR]")+" "+msg)
}

// ---------------------------------------------------------------------------
// Execução (equivalente ao set -euo pipefail do original)
// ---------------------------------------------------------------------------

// run executa um comando com a saída em tempo real. Falha não tratada encerra
// o programa com o mesmo código de saída do comando (como faria set -e).
func run(nome string, args ...string) {
	cmd := exec.Command(nome, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
}

// runTrue executa um comando protegido por "|| true" no original: a falha é
// ignorada (a saída continua em tempo real).
func runTrue(nome string, args ...string) {
	cmd := exec.Command(nome, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = cmd.Run()
}

// morrer reproduz a saída do set -e: sai com o código do comando que falhou.
func morrer(err error) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	logError(err.Error())
	os.Exit(1)
}

// espelharLog reproduz fielmente o "exec > >(tee -a LOG) 2>&1" do original:
// a partir daqui stdout e stderr vão para o console E, em append, para o
// arquivo de log — inclusive a saída dos subprocessos, que herdam os fds,
// exatamente como no script bash.
func espelharLog(logPath string) {
	r, w, err := os.Pipe()
	if err != nil {
		morrer(err)
	}
	tee := exec.Command("tee", "-a", logPath)
	tee.Stdin = r
	tee.Stdout = os.Stdout
	tee.Stderr = os.Stderr
	if err := tee.Start(); err != nil {
		morrer(err)
	}
	r.Close()
	os.Stdout = w
	os.Stderr = w
}

// ---------------------------------------------------------------------------
// Constantes
// ---------------------------------------------------------------------------

const logPath = "/var/log/osrhel-bootstrap.log"

// ---------------------------------------------------------------------------
// Aplicação
// ---------------------------------------------------------------------------

func main() {
	prepararSaida()

	// Check root
	if os.Geteuid() != 0 {
		logError("This script must be run as root")
		os.Exit(1)
	}

	espelharLog(logPath)

	logInfo("Starting RHEL-like system bootstrap")
	// "Date: $(date)": no bash a falha do $(...) não aborta o script.
	if data, err := exec.Command("date").Output(); err == nil {
		logInfo("Date: " + strings.TrimSpace(string(data)))
	}

	enableRepos()
	installPackages()
	enableServices()
	basicTuning()
	finalize()
}

func enableRepos() {
	logInfo("Enabling essential repositories")
	run("dnf", "install", "-y", "epel-release")
	run("dnf", "install", "-y", "dnf-plugins-core")
	run("dnf", "makecache")
}

func installPackages() {
	logInfo("Installing essential packages")

	pkgs := []string{
		// Network / DNS
		"bind-utils", "iproute", "net-tools", "nmap", "tcpdump", "traceroute", "mtr", "ethtool", "NetworkManager",
		// System / Debug
		"lsof", "strace", "htop", "iotop", "sysstat", "psmisc", "procps-ng",
		// Security
		"fail2ban", "audit", "firewalld", "policycoreutils-python-utils",
		// Automation / DevOps
		"git", "curl", "wget", "jq", "rsync", "bash-completion",
		// Compression / Utilities
		"zip", "unzip", "tar", "gzip", "bzip2", "xz", "vim-enhanced", "nano", "tree", "screen", "tmux",
	}

	run("dnf", append([]string{"install", "-y"}, pkgs...)...)
}

func enableServices() {
	logInfo("Enabling important services")
	runTrue("systemctl", "enable", "--now", "firewalld")
	runTrue("systemctl", "enable", "--now", "auditd")
	runTrue("systemctl", "enable", "--now", "fail2ban")
}

func basicTuning() {
	logInfo("Applying basic tuning")
	// Example: Increase file descriptors
	// Fidelidade ao original: o append NÃO é idempotente — as duas linhas são
	// duplicadas a cada execução, exatamente como no script bash.
	anexarLimits("* soft nofile 65535")
	anexarLimits("* hard nofile 65535")
}

// anexarLimits equivale ao `echo "<linha>" >> /etc/security/limits.conf`.
func anexarLimits(linha string) {
	f, err := os.OpenFile("/etc/security/limits.conf", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		morrer(err)
	}
	defer f.Close()
	if _, err := f.WriteString(linha + "\n"); err != nil {
		morrer(err)
	}
}

func finalize() {
	logInfo("Finalizing configuration")
	run("dnf", "autoremove", "-y")
	logInfo("Bootstrap completed successfully")
}

// --- fim do arquivo polyglot (osrhel.go/osrhel.sh) ---

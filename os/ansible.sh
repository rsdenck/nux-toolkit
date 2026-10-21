/* 2>/dev/null
# ---------------------------------------------------------------------------
#  ansible.sh — polyglot: este mesmo arquivo é ansible.go (Go) e ansible.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run ansible.go [argumentos]
#    Bash ...: bash ansible.sh  [argumentos]   (exige a toolchain Go)
#
#  Project: GETDEV
#  Script: ansible.sh
#  Description: Full Ansible LTS bootstrap for RHEL-like systems
#  Author: ranlens.denck
#  Category: os
#  Compatibility: Rocky Linux, AlmaLinux, CentOS Stream, RHEL
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[ansible.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[ansible.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-ansible"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/ansible.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init ansible >/dev/null 2>&1) || :

_bin="$_trab/ansible"
if [ ! -x "$_bin" ] || [ "$_trab/ansible.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando ansible.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o ansible.tmp ansible.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o ansible.tmp ansible.go) ||
			_die "falha ao compilar $_trab/ansible.go"
	}
	mv -f "$_trab/ansible.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (ansible.go/ansible.sh) ---'
*/
// ansible.go — bootstrap completo do Ansible LTS em RHEL-like.
//
// Porta fiel do script original ansible.sh (GETDEV): exige root, espelha stdout
// e stderr em /var/log/ansible-bootstrap.log (append) e no console, valida a
// distro pelo ID do /etc/os-release (rocky, almalinux, centos ou rhel),
// instala as dependências base via dnf, habilita o EPEL (guardado por rpm -q),
// instala o ansible-core e — quando o pacote "ansible" existe no repositório —
// também o ansible, e confere a instalação com "ansible --version". Nenhum
// texto de saída foi traduzido.
//
// Execução:
//
//	go run ansible.go
//	bash ansible.sh          (mesmo arquivo, polyglot Go + sh)
package main

import (
	"errors"
	"fmt"
	"io"
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

// okExec corresponde aos guards "if cmd; then" / "cmd >/dev/null 2>&1" do
// original: devolve true quando o comando termina com código 0, sem ecoar saída.
func okExec(nome string, args ...string) bool {
	cmd := exec.Command(nome, args...)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run() == nil
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
// Constantes e estado
// ---------------------------------------------------------------------------

const logPath = "/var/log/ansible-bootstrap.log"

var (
	osID       string
	prettyName string
)

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

	logInfo("Starting Ansible bootstrap")

	detectOS()
	installBasePackages()
	enableEPEL()
	installAnsible()
	verify()
}

// detectOS espelha o "source /etc/os-release" + case do original.
func detectOS() {
	dados, err := os.ReadFile("/etc/os-release")
	if err != nil {
		logError("Could not detect operating system.")
		os.Exit(1)
	}
	for _, ln := range strings.Split(string(dados), "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "ID=") {
			osID = strings.Trim(strings.TrimPrefix(ln, "ID="), `"'`)
		} else if strings.HasPrefix(ln, "PRETTY_NAME=") {
			prettyName = strings.Trim(strings.TrimPrefix(ln, "PRETTY_NAME="), `"'`)
		}
	}

	switch osID {
	case "rocky", "almalinux", "centos", "rhel":
		logInfo("System detected: " + prettyName)
	default:
		logError("System not supported: " + osID)
		os.Exit(1)
	}
}

func installBasePackages() {
	logInfo("Installing base packages...")
	run("dnf", "install", "-y",
		"ca-certificates",
		"curl",
		"wget",
		"git",
		"rsync",
		"tar",
		"unzip",
		"python3",
		"python3-pip")
}

func enableEPEL() {
	if okExec("rpm", "-q", "epel-release") {
		logInfo("EPEL is already installed.")
		return
	}
	logInfo("Enabling EPEL repository...")
	run("dnf", "install", "-y", "epel-release")
}

func installAnsible() {
	logInfo("Installing Ansible LTS...")

	// ansible-core is the recommended and maintained package
	run("dnf", "install", "-y", "ansible-core")

	// ansible package (default collections)
	if okExec("dnf", "list", "ansible") {
		run("dnf", "install", "-y", "ansible")
	}
}

func verify() {
	logInfo("Verifying installation...")
	if _, err := exec.LookPath("ansible"); err != nil {
		logError("Ansible installation failed.")
		os.Exit(1)
	}
	run("ansible", "--version")
	logInfo("Ansible installed successfully.")
}

// --- fim do arquivo polyglot (ansible.go/ansible.sh) ---

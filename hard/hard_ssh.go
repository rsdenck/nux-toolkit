/* 2>/dev/null
# ---------------------------------------------------------------------------
#  hard_ssh.sh — polyglot: este mesmo arquivo é hard_ssh.go (Go) e hard_ssh.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run hard_ssh.go [argumentos]
#    Bash ...: bash hard_ssh.sh  [argumentos]   (exige a toolchain Go)
#
# ------------------------------------------------------------------------------------
# Projeto: GETDEV
# Script: hard_ssh.sh
# Description: SSH and Audit hardening (Migrated from APEX-HARDENING)
# Author: ranlens.denck
# Category: hard
# Compatibility: Ubuntu, Debian, CentOS, Fedora, Arch
# ------------------------------------------------------------------------------------
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[hard_ssh.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[hard_ssh.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-hard_ssh"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/hard_ssh.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init hard_ssh >/dev/null 2>&1) || :

_bin="$_trab/hard_ssh"
if [ ! -x "$_bin" ] || [ "$_trab/hard_ssh.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando hard_ssh.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o hard_ssh.tmp hard_ssh.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o hard_ssh.tmp hard_ssh.go) ||
			_die "falha ao compilar $_trab/hard_ssh.go"
	}
	mv -f "$_trab/hard_ssh.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export HARD_SSH_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (hard_ssh.go/hard_ssh.sh) ---'
*/
// hard_ssh.go — SSH and Audit hardening (Migrated from APEX-HARDENING).
//
// Projeto: GETDEV
// Script: hard_ssh.sh
// Author: ranlens.denck
// Category: hard
// Compatibility: Ubuntu, Debian, CentOS, Fedora, Arch
//
// Execução:
//
//	go run hard_ssh.go [argumentos]
//	bash hard_ssh.sh  [argumentos]
package main

import (
	"fmt"
	"os"
	"os/exec"
)

const (
	colorGreen = "\033[0;32m"
	colorRed   = "\033[0;31m"
	colorNC    = "\033[0m"
	sshPortNew = "4022"
)

func logInfo(msg string) {
	if isTTY() && noColor() == "" {
		fmt.Printf("%s[INFO]%s %s\n", colorGreen, colorNC, msg)
		return
	}
	fmt.Printf("[INFO] %s\n", msg)
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
		logErr("This script must be run as root.")
		os.Exit(1)
	}
}

func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func installAuditd() {
	logInfo("Installing/Checking auditd...")
	if commandExists("apt-get") {
		runOrExit("apt-get", "update")
		runOrExit("apt-get", "install", "-y", "auditd")
	} else if commandExists("yum") {
		runOrExit("yum", "install", "-y", "audit")
	}
	runOrExit("systemctl", "enable", "--now", "auditd")
}

// runOrExit executa um comando com a mesma saída do original (herdada, o
// original não redireciona nada) e reproduz o `set -euo pipefail`: falha
// não tratada aborta com o próprio código do comando, como o bash faria.
func runOrExit(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(exitCode(err))
	}
}

// runTry executa ignorando o resultado, mantendo a saída visível — usado no
// lado esquerdo de `systemctl restart sshd || systemctl restart ssh`.
func runTry(name string, args ...string) bool {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run() == nil
}

func exitCode(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		if c := ee.ExitCode(); c >= 0 {
			return c
		}
	}
	return 1
}

func configureSSH() {
	logInfo(fmt.Sprintf("Configuring SSH on port %s...", sshPortNew))
	// [ -f /etc/ssh/sshd_config ] || return (qualquer falha de stat → return)
	if fi, err := os.Stat("/etc/ssh/sshd_config"); err != nil || !fi.Mode().IsRegular() {
		return
	}
	// Os quatro `sed -i` do original, verbatim — preservam dono/modo do
	// arquivo e as bordas exatas das expressões (sem reimplementar em Go).
	runOrExit("sed", "-i", "s/^#Port 22/Port "+sshPortNew+"/", "/etc/ssh/sshd_config")
	runOrExit("sed", "-i", "s/^Port .*/Port "+sshPortNew+"/", "/etc/ssh/sshd_config")
	runOrExit("sed", "-i", "s/^#PermitRootLogin.*/PermitRootLogin no/", "/etc/ssh/sshd_config")
	runOrExit("sed", "-i", "s/^PasswordAuthentication yes/PasswordAuthentication no/", "/etc/ssh/sshd_config")
	// systemctl restart sshd || systemctl restart ssh
	if runTry("systemctl", "restart", "sshd") {
		return
	}
	runOrExit("systemctl", "restart", "ssh")
}

func applyHardening() {
	checkRoot()
	installAuditd()
	configureSSH()
	logInfo("SSH hardening completed.")
}

func main() {
	// case "${1:-}" in --apply): só o $1 importa, argumentos extras são
	// ignorados como no original.
	if len(os.Args) >= 2 && os.Args[1] == "--apply" {
		applyHardening()
		return
	}
	fmt.Print("Usage: hard_ssh.sh --apply\n")
	os.Exit(1)
}

// --- fim do arquivo polyglot (hard_ssh.go/hard_ssh.sh) ---

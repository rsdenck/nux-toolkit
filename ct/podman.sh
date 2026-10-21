/* 2>/dev/null
# ---------------------------------------------------------------------------
#  podman.sh — polyglot: este mesmo arquivo é podman.go (Go) e podman.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run podman.go [argumentos]
#    Bash ...: bash podman.sh  [argumentos]   (exige a toolchain Go)
#
# ------------------------------------------------------------------------------------
# Projeto: GETDEV
# Script: podman_stack.sh
# Description: Podman installation and rootless configuration
# Author: ranlens.denck
# Category: ct
# Compatibility: Ubuntu, Debian, CentOS, Fedora, Arch
# ------------------------------------------------------------------------------------
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[podman.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[podman.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-podman"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/podman.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init podman >/dev/null 2>&1) || :

_bin="$_trab/podman"
if [ ! -x "$_bin" ] || [ "$_trab/podman.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando podman.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o podman.tmp podman.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o podman.tmp podman.go) ||
			_die "falha ao compilar $_trab/podman.go"
	}
	mv -f "$_trab/podman.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export PODMAN_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (podman.go/podman.sh) ---'
*/
// podman.go — Podman installation and rootless configuration.
//
// Projeto: GETDEV
// Script: podman_stack.sh (renomeado para podman.sh)
// Author: ranlens.denck
// Category: ct
// Compatibility: Ubuntu, Debian, CentOS, Fedora, Arch
//
// Execução:
//
//	go run podman.go [argumentos]
//	bash podman.sh  [argumentos]
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	colorGreen = "\033[0;32m"
	colorRed   = "\033[0;31m"
	colorNC    = "\033[0m"
)

var (
	distro string
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

func detectDistro() {
	if _, err := os.Stat("/etc/os-release"); err == nil {
		f, err := os.Open("/etc/os-release")
		if err == nil {
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "ID=") {
					id := strings.TrimPrefix(line, "ID=")
					id = strings.Trim(id, `"`)
					distro = id
					break
				}
			}
		}
	}
}

// run executa um comando com a saída herdada (o original não redireciona) e
// aborta em falha — o podman_stack.sh roda sob set -euo pipefail.
func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(exitCode(err))
	}
}

func exitCode(err error) int {
	if ee, ok := err.(*exec.ExitError); ok {
		if c := ee.ExitCode(); c >= 0 {
			return c
		}
	}
	return 1
}

func installPodman() {
	logInfo("Installing Podman stack...")
	detectDistro()
	switch distro {
	case "ubuntu", "debian":
		run("apt-get", "update")
		run("apt-get", "install", "-y", "podman", "buildah", "skopeo")
	case "centos", "rhel", "fedora":
		run("yum", "install", "-y", "podman", "buildah", "skopeo")
	case "arch":
		run("pacman", "-S", "--noconfirm", "podman", "buildah", "skopeo")
	default:
		// distro sem ramo no case do original ainda imprime sucesso (fiel)
	}
	logInfo("Podman stack installed.")
}

func main() {
	checkRoot()
	installPodman()
}

// --- fim do arquivo polyglot (podman.go/podman.sh) ---

/* 2>/dev/null
# ---------------------------------------------------------------------------
#  docker.sh — polyglot: este mesmo arquivo é docker.go (Go) e docker.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run docker.go [argumentos]
#    Bash ...: bash docker.sh  [argumentos]   (exige a toolchain Go)
#
# ------------------------------------------------------------------------------------
# Projeto: GETDEV
# Script: docker_stack.sh
# Description: Professional Docker & Docker Compose installation and optimization
# Author: ranlens.denck
# Category: ct
# Compatibility: Ubuntu, Debian, CentOS, Fedora
# ------------------------------------------------------------------------------------
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[docker.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[docker.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-docker"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/docker.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init docker >/dev/null 2>&1) || :

_bin="$_trab/docker"
if [ ! -x "$_bin" ] || [ "$_trab/docker.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando docker.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o docker.tmp docker.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o docker.tmp docker.go) ||
			_die "falha ao compilar $_trab/docker.go"
	}
	mv -f "$_trab/docker.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export DOCKER_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (docker.go/docker.sh) ---'
*/
// docker.go — Professional Docker & Docker Compose installation and optimization.
//
// Projeto: GETDEV
// Script: docker_stack.sh (renomeado para docker.sh)
// Author: ranlens.denck
// Category: ct
// Compatibility: Ubuntu, Debian, CentOS, Fedora
//
// Execução:
//
//	go run docker.go [argumentos]
//	bash docker.sh  [argumentos]
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
			id := ""
			name := ""
			version := ""
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "ID=") {
					id = strings.TrimPrefix(line, "ID=")
					id = strings.Trim(id, `"`)
					distro = id
				} else if strings.HasPrefix(line, "NAME=") {
					name = strings.TrimPrefix(line, "NAME=")
					name = strings.Trim(name, `"`)
				} else if strings.HasPrefix(line, "VERSION_CODENAME=") {
					version = strings.TrimPrefix(line, "VERSION_CODENAME=")
					version = strings.Trim(version, `"`)
				}
			}
			_ = name
			if version != "" {
				os.Setenv("VERSION_CODENAME", version)
			}
		}
	}
}

// run executa um comando com a saída herdada (o original não redireciona) e
// aborta em falha — o docker_stack.sh roda sob set -euo pipefail.
func run(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(exitCode(err))
	}
}

// runPipe roda um pipeline verbatim (ex.: curl | gpg). O original tem
// `set -o pipefail`, então usa bash quando existe; sem bash, cai para sh -c
// como manda o guia.
func runPipe(comando string) {
	var c *exec.Cmd
	if _, err := exec.LookPath("bash"); err == nil {
		c = exec.Command("bash", "-c", "set -o pipefail; "+comando)
	} else {
		c = exec.Command("sh", "-c", comando)
	}
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
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

func installDocker() {
	logInfo("Detecting distribution and installing Docker...")
	detectDistro()
	switch distro {
	case "ubuntu", "debian":
		run("apt-get", "update")
		run("apt-get", "install", "-y", "ca-certificates", "curl", "gnupg")
		run("install", "-m", "0755", "-d", "/etc/apt/keyrings")
		// curl | gpg as in original
		runPipe(fmt.Sprintf("curl -fsSL https://download.docker.com/linux/%s/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg", distro))
		run("chmod", "a+r", "/etc/apt/keyrings/docker.gpg")
		// Linha do repositório verbatim do original:
		// echo "deb [arch=$(dpkg --print-architecture) signed-by=…] https://download.docker.com/linux/$ID $VERSION_CODENAME stable" | tee … > /dev/null
		verCodename := os.Getenv("VERSION_CODENAME")
		runPipe(fmt.Sprintf(
			"echo \"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/%s %s stable\" | tee /etc/apt/sources.list.d/docker.list > /dev/null",
			distro, verCodename))
		run("apt-get", "update")
		run("apt-get", "install", "-y", "docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin")
	case "centos", "rhel", "fedora":
		run("yum", "install", "-y", "yum-utils")
		runPipe(fmt.Sprintf("yum-config-manager --add-repo https://download.docker.com/linux/%s/docker-ce.repo", distro))
		run("yum", "install", "-y", "docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin")
	default:
		logErr("Unsupported distribution for this automated script.")
		os.Exit(1)
	}
	run("systemctl", "enable", "--now", "docker")
	logInfo("Docker Stack installed and running.")
}

func optimizeDocker() {
	logInfo("Optimizing Docker daemon settings...")
	os.MkdirAll("/etc/docker", 0755)
	daemon := `{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m",
    "max-file": "3"
  },
  "max-concurrent-downloads": 10,
  "max-concurrent-uploads": 5
}
`
	if err := os.WriteFile("/etc/docker/daemon.json", []byte(daemon), 0644); err != nil {
		// o redirecionamento do heredoc original falhando aborta via set -e
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	run("systemctl", "restart", "docker")
}

func main() {
	checkRoot()
	installDocker()
	optimizeDocker()
}

// --- fim do arquivo polyglot (docker.go/docker.sh) ---

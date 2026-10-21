/* 2>/dev/null
# ---------------------------------------------------------------------------
#  node.sh — polyglot: este mesmo arquivo é node.go (Go) e node.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run node.go [argumentos]
#    Bash ...: bash node.sh  [argumentos]   (exige a toolchain Go)
#
#  Projeto: GETDEV
#  Script: node.sh
#  Description: Node.js Frontend environment setup (NVM, Node LTS, Yarn, PNPM)
#  Author: ranlens.denck
#  Category: dev
#  Compatibility: Ubuntu, Debian, CentOS, Fedora, Arch
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[node.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[node.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-node"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/node.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init node >/dev/null 2>&1) || :

_bin="$_trab/node"
if [ ! -x "$_bin" ] || [ "$_trab/node.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando node.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o node.tmp node.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o node.tmp node.go) ||
			_die "falha ao compilar $_trab/node.go"
	}
	mv -f "$_trab/node.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (node.go/node.sh) ---'
*/
// node.go — configuração do ambiente frontend Node.js (NVM, Node LTS, Yarn, PNPM).
//
// Porta fiel do script original node_fe.sh (GETDEV): detecta a distro, instala
// as dependências básicas (curl, git, build tools), o NVM, o Node.js LTS via
// nvm e, por fim, yarn e pnpm globalmente com npm. Nenhum texto de saída foi
// traduzido ou alterado.
//
// Execução:
//
//	go run node.go
//	bash node.sh          (mesmo arquivo, polyglot Go + sh)
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func logInfo(format string, a ...interface{}) {
	fmt.Println(pintar(cVerde, "[INFO]") + " " + fmt.Sprintf(format, a...))
}

func logErr(format string, a ...interface{}) {
	fmt.Println(pintar(cVerm, "[ERROR]") + " " + fmt.Sprintf(format, a...))
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

// shc roda um trecho de shell verbatim do original (pipelines, &&, source).
func shc(script string) {
	run("sh", "-c", script)
}

// morrer reproduz a saída do set -e: sai com o código do comando que falhou.
func morrer(err error) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	logErr("%v", err)
	os.Exit(1)
}

// ---------------------------------------------------------------------------
// Distro
// ---------------------------------------------------------------------------

var distro string

// detectDistro espelha o original: lê o ID do /etc/os-release ou "unknown".
func detectDistro() {
	distro = "unknown"
	dados, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return
	}
	for _, ln := range strings.Split(string(dados), "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "ID=") {
			distro = strings.Trim(strings.TrimPrefix(strings.TrimSpace(ln), "ID="), `"'`)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Instalação
// ---------------------------------------------------------------------------

const urlNVMInstall = "curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash"

var nvmDir string

func installDeps() {
	logInfo("Installing basic dependencies...")
	switch distro {
	case "ubuntu", "debian":
		shc("apt-get update && apt-get install -y curl git build-essential")
	case "centos", "rhel", "fedora":
		shc(`yum groupinstall -y "Development Tools" && yum install -y curl git`)
	case "arch":
		shc("pacman -S --noconfirm base-devel curl git")
	}
}

func installNVM() {
	if fi, err := os.Stat(nvmDir); err == nil && fi.IsDir() {
		logInfo("NVM already installed.")
		return
	}
	logInfo("Installing NVM...")
	shc(urlNVMInstall)
	shc(`set -e; export NVM_DIR="$HOME/.nvm"; [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"`)
}

func installNode() {
	logInfo("Installing Node.js LTS...")
	shc(`set -e; export NVM_DIR="$HOME/.nvm"; [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"; nvm install --lts; nvm use --lts; node -v`)
}

func installManagers() {
	logInfo("Installing Yarn and PNPM...")
	shc(`set -e; export NVM_DIR="$HOME/.nvm"; [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"; npm install -g yarn pnpm`)
}

func main() {
	prepararSaida()
	home, err := os.UserHomeDir()
	if err != nil {
		logErr("%v", err)
		os.Exit(1)
	}
	nvmDir = filepath.Join(home, ".nvm")

	detectDistro()
	installDeps()
	installNVM()
	installNode()
	installManagers()
	logInfo("Frontend Node.js environment configured successfully!")
}

// --- fim do arquivo polyglot (node.go/node.sh) ---

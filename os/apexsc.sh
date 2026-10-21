/* 2>/dev/null
# ---------------------------------------------------------------------------
#  apexsc.sh — polyglot: este mesmo arquivo é apexsc.go (Go) e apexsc.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run apexsc.go [argumentos]
#    Bash ...: bash apexsc.sh  [argumentos]   (exige a toolchain Go)
#
#  Projeto: GETDEV
#  Script: apexsc.sh
#  Description: Apex Secure Connectivity Bootstrap Script
#  Author: ranlens.denck
#  Category: os
#  Compatibility: Ubuntu, Debian, RHEL, Rocky, Alma, CentOS
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[apexsc.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[apexsc.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-apexsc"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/apexsc.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init apexsc >/dev/null 2>&1) || :

_bin="$_trab/apexsc"
if [ ! -x "$_bin" ] || [ "$_trab/apexsc.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando apexsc.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o apexsc.tmp apexsc.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o apexsc.tmp apexsc.go) ||
			_die "falha ao compilar $_trab/apexsc.go"
	}
	mv -f "$_trab/apexsc.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (apexsc.go/apexsc.sh) ---'
*/
// apexsc.go — bootstrap de conectividade segura (cloudflared + tailscale).
//
// Porte fiel do script original apexsc.sh (GETDEV): detecta a distro pelo
// /etc/os-release (ID/ID_LIKE) escolhendo apt ou dnft (outra distro aborta
// como no fonte), instala as dependências base (incluindo pipx com
// `pipx ensurepath || true`), o cloudflared (repo + keyring no Debian; rpm
// direto no RHEL) e o tailscale via pipeline oficial. Nenhum texto de saída
// foi traduzido ou alterado.
//
// Quirks fiéis ao original (mantidos de propósito):
//
//   - ARCH="$(uname -m)" é calculado e nunca usado no original — omitido aqui;
//   - o rpm do cloudflared está hardcoded como cloudflared-linux-x86_64.rpm,
//     mesmo em máquinas de outra arquitetura (fiel ao fonte);
//   - no RHEL o rpm é baixado para o diretório atual (cwd) e removido depois.
//
// Execução:
//
//	go run apexsc.go
//	bash apexsc.sh          (mesmo arquivo, polyglot Go + sh)
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Global Variables
var pkgManager string

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

func logError(format string, a ...interface{}) {
	fmt.Println(pintar(cVerm, "[ERROR]") + " " + fmt.Sprintf(format, a...))
}

// ---------------------------------------------------------------------------
// Execução (equivalente ao set -euo pipefail do original)
// ---------------------------------------------------------------------------

// morrer reproduz a saída do set -e: sai com o código do comando que falhou.
func morrer(err error) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	logError("%v", err)
	os.Exit(1)
}

// shc roda um trecho de shell verbatim do original (pipelines, redirecionamentos,
// $(lsb_release -cs)); falha não tratada encerra o programa como faria set -e.
func shc(script string) {
	cmd := exec.Command("sh", "-c", script)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
}

func cmdExists(nome string) bool {
	_, err := exec.LookPath(nome)
	return err == nil
}

// osRelease lê uma chave do /etc/os-release (equivalente ao `. /etc/os-release`
// do original, restrito às chaves realmente usadas).
func osRelease(chave string) string {
	dados, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(dados), "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, chave+"=") {
			return strings.Trim(strings.TrimPrefix(ln, chave+"="), `"'`)
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Funções do original
// ---------------------------------------------------------------------------

// Detect system
func detectOS() {
	if _, err := os.Stat("/etc/os-release"); err != nil {
		logError("Could not detect operating system.")
		os.Exit(1)
	}

	id := osRelease("ID")
	idLike := osRelease("ID_LIKE")

	// [[ =~ (debian|ubuntu) ]] etc. — alternativas literais sem âncora,
	// equivalentes a Contains.
	switch {
	case strings.Contains(id, "debian") || strings.Contains(id, "ubuntu") ||
		strings.Contains(idLike, "debian"):
		pkgManager = "apt"
	case strings.Contains(id, "rhel") || strings.Contains(id, "rocky") ||
		strings.Contains(id, "almalinux") || strings.Contains(id, "centos") ||
		strings.Contains(idLike, "rhel"):
		pkgManager = "dnf"
	default:
		logError("Distribution not supported: %s", id)
		os.Exit(1)
	}

	logInfo("System detected: %s", osRelease("PRETTY_NAME"))
	logInfo("Package manager: %s", pkgManager)
}

// Base dependencies
func installBaseDeps() {
	logInfo("Installing base dependencies")

	if pkgManager == "apt" {
		shc("apt update -y")
		shc("apt install -y curl gnupg ca-certificates lsb-release python3-pip pipx")
	} else {
		shc("dnf install -y curl gnupg2 ca-certificates python3-pip pipx")
	}

	// Ensure pipx is in path for the current shell session
	// (export PATH="$PATH:$HOME/.local/bin"; pipx ensurepath || true)
	cmd := exec.Command("pipx", "ensurepath")
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH")+":"+os.Getenv("HOME")+"/.local/bin")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = cmd.Run() // || true
}

// Cloudflared installation
func installCloudflared() {
	if cmdExists("cloudflared") {
		logInfo("cloudflared already installed.")
		return
	}

	logInfo("Installing cloudflared")

	if pkgManager == "apt" {
		shc("curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg | gpg --dearmor -o /usr/share/keyrings/cloudflare.gpg")
		shc(`echo "deb [signed-by=/usr/share/keyrings/cloudflare.gpg] https://pkg.cloudflare.com/cloudflared $(lsb_release -cs) main" > /etc/apt/sources.list.d/cloudflared.list`)
		shc("apt update -y")
		shc("apt install -y cloudflared")
	} else {
		// For RHEL-like systems, cloudflared provides a direct rpm
		// (fiel: x86_64 hardcoded no original, mesmo com $ARCH calculado)
		shc("curl -L --output cloudflared.rpm https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-x86_64.rpm")
		shc("dnf localinstall -y cloudflared.rpm")
		shc("rm -f cloudflared.rpm")
	}
}

// Tailscale installation
func installTailscale() {
	if cmdExists("tailscale") {
		logInfo("tailscale already installed.")
		return
	}

	logInfo("Installing tailscale")
	shc("curl -fsSL https://tailscale.com/install.sh | sh")
}

// ---------------------------------------------------------------------------
// Aplicação
// ---------------------------------------------------------------------------

func main() {
	prepararSaida()

	// Main execution
	if os.Geteuid() != 0 {
		logError("This script must be run as root")
		os.Exit(1)
	}

	detectOS()
	installBaseDeps()
	installCloudflared()
	installTailscale()

	logInfo("Apex Secure Connectivity setup completed!")
}

// --- fim do arquivo polyglot (apexsc.go/apexsc.sh) ---

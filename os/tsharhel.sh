/* 2>/dev/null
# ---------------------------------------------------------------------------
#  tsharhel.sh — polyglot: este mesmo arquivo é tsharhel.go (Go) e tsharhel.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run tsharhel.go [argumentos]
#    Bash ...: bash tsharhel.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      tsharhel.sh
#  Description: Wireshark CLI (tshark) Bootstrap for RHEL-like systems
#  Author:      GETDEV Team
#  Category:    Network / Security
#  Compatibility: Rocky Linux, AlmaLinux, CentOS, RHEL
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[tsharhel.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[tsharhel.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-tsharhel"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/tsharhel.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init tsharhel >/dev/null 2>&1) || :

_bin="$_trab/tsharhel"
if [ ! -x "$_bin" ] || [ "$_trab/tsharhel.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando tsharhel.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o tsharhel.tmp tsharhel.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o tsharhel.tmp tsharhel.go) ||
			_die "falha ao compilar $_trab/tsharhel.go"
	}
	mv -f "$_trab/tsharhel.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (tsharhel.go/tsharhel.sh) ---'
*/
// tsharhel.go — bootstrap do tshark (CLI do Wireshark) para sistemas RHEL-like.
//
// Porta fiel do script original tsharhel.sh (GETDEV): exige root, confere os
// comandos obrigatórios (dnf, rpm, curl), valida o sistema (rocky, almalinux,
// centos ou rhel), instala as dependências base, habilita o EPEL quando
// "rpm -q epel-release" não encontra o pacote, instala o wireshark-cli via
// dnf, concede as capabilities de captura com setcap no caminho resolvido do
// tshark (melhor esforço) e confere com "tshark --version".
//
// Execução:
//
//	go run tsharhel.go
//	bash tsharhel.sh        (mesmo arquivo, polyglot Go + sh)
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
// Constantes (fiéis ao original)
// ---------------------------------------------------------------------------

var requiredCmds = []string{"dnf", "rpm", "curl"}

const wiresharkCmd = "tshark"

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

func logInfo(msg string)  { fmt.Println(pintar(cVerde, "[INFO]") + " " + msg) }
func logError(msg string) { fmt.Println(pintar(cVerm, "[ERROR]") + " " + msg) }

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

// morrer reproduz a saída do set -e: sai com o código do comando que falhou.
func morrer(err error) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	logError(err.Error())
	os.Exit(1)
}

// disponivel reproduz: comando ... >/dev/null 2>&1 dentro de um if.
func disponivel(nome string, args ...string) bool {
	cmd := exec.Command(nome, args...)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run() == nil
}

// arquivo reproduz o teste [[ -f arquivo ]].
func arquivo(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular()
}

// osRelease lê uma variável de /etc/os-release (o ". /etc/os-release" original).
func osRelease(chave string) string {
	dados, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(dados), "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, chave+"=") {
			continue
		}
		return strings.Trim(strings.TrimPrefix(ln, chave+"="), `"'`)
	}
	return ""
}

// ---------------------------------------------------------------------------
// Etapas do bootstrap (mesma ordem do original)
// ---------------------------------------------------------------------------

func checkRoot() {
	if os.Geteuid() != 0 {
		logError("This script must be run as root.")
		os.Exit(1)
	}
}

func checkCommands() {
	for _, cmd := range requiredCmds {
		if _, err := exec.LookPath(cmd); err != nil {
			logError("Required command not found: " + cmd)
			os.Exit(1)
		}
	}
}

func detectOS() {
	if !arquivo("/etc/os-release") {
		logError("Could not detect operating system.")
		os.Exit(1)
	}

	id := osRelease("ID")
	switch id {
	case "rocky", "almalinux", "centos", "rhel":
		logInfo("System detected: " + osRelease("PRETTY_NAME"))
	default:
		logError("Unsupported system: " + id)
		os.Exit(1)
	}
}

func installDependencies() {
	logInfo("Installing base dependencies...")
	run("dnf", "install", "-y",
		"ca-certificates",
		"curl",
		"dnf-plugins-core")
}

func enableEPEL() {
	if !disponivel("rpm", "-q", "epel-release") {
		logInfo("Enabling official EPEL repository...")
		run("dnf", "install", "-y", "epel-release")
	} else {
		logInfo("EPEL repository already enabled.")
	}
}

func installTshark() {
	logInfo("Installing Wireshark CLI (tshark)...")
	run("dnf", "install", "-y", "wireshark-cli")
}

func configureCapabilities() {
	if _, err := exec.LookPath("setcap"); err == nil {
		logInfo("Configuring capabilities for non-root capture...")

		// "$(command -v tshark)" — caminho resolvido, como no original
		tsharkPath := ""
		if p, err := exec.LookPath("tshark"); err == nil {
			tsharkPath = p
		}
		// setcap cap_net_raw,cap_net_admin+eip ... || true
		if err := exec.Command("setcap", "cap_net_raw,cap_net_admin+eip", tsharkPath).Run(); err != nil {
			// o original silencia a falha com || true
		}
	}
}

func verifyInstallation() {
	logInfo("Verifying tshark installation...")

	if _, err := exec.LookPath(wiresharkCmd); err != nil {
		logError("tshark was not installed correctly.")
		os.Exit(1)
	}

	fmt.Println("========================================")
	logInfo("Wireshark CLI (tshark) installed successfully")
	fmt.Println("========================================")
	run("tshark", "--version")
	fmt.Println("========================================")
}

func main() {
	prepararSaida()
	checkRoot()
	checkCommands()
	detectOS()
	installDependencies()
	enableEPEL()
	installTshark()
	configureCapabilities()
	verifyInstallation()
}

// --- fim do arquivo polyglot (tsharhel.go/tsharhel.sh) ---

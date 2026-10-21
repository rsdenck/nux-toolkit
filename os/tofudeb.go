/* 2>/dev/null
# ---------------------------------------------------------------------------
#  tofudeb.sh — polyglot: este mesmo arquivo é tofudeb.go (Go) e tofudeb.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run tofudeb.go [argumentos]
#    Bash ...: bash tofudeb.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      tofudeb.sh
#  Description: OpenTofu (LTS) Bootstrap for Debian-like systems
#  Author:      GETDEV Team
#  Category:    Automation / IaC
#  Compatibility: Debian, Ubuntu, Linux Mint, ZorinOS
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[tofudeb.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[tofudeb.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-tofudeb"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/tofudeb.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init tofudeb >/dev/null 2>&1) || :

_bin="$_trab/tofudeb"
if [ ! -x "$_bin" ] || [ "$_trab/tofudeb.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando tofudeb.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o tofudeb.tmp tofudeb.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o tofudeb.tmp tofudeb.go) ||
			_die "falha ao compilar $_trab/tofudeb.go"
	}
	mv -f "$_trab/tofudeb.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (tofudeb.go/tofudeb.sh) ---'
*/
// tofudeb.go — bootstrap do OpenTofu (LTS) para sistemas Debian-like.
//
// Porta fiel do script original tofudeb.sh (GETDEV): exige root, confere os
// comandos obrigatórios (apt, curl, gpg, lsb_release), valida o sistema
// (debian, ubuntu, linuxmint ou zorin), instala as dependências base, importa
// a chave GPG oficial e grava /etc/apt/sources.list.d/opentofu.list (com
// $(lsb_release -cs)), ambos com guarda de idempotência, e instala o tofu via
// apt; confere com "tofu version".
//
// Execução:
//
//	go run tofudeb.go
//	bash tofudeb.sh        (mesmo arquivo, polyglot Go + sh)
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ---------------------------------------------------------------------------
// Constantes (fiéis ao original)
// ---------------------------------------------------------------------------

var requiredCmds = []string{"apt", "curl", "gpg", "lsb_release"}

const (
	opentofuKeyring = "/usr/share/keyrings/opentofu-archive-keyring.gpg"
	opentofuList    = "/etc/apt/sources.list.d/opentofu.list"
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

// shc roda um trecho verbatim do shell original (bash com pipefail; sh como
// fallback) com a saída em tempo real. Falha não tratada encerra o programa
// com o mesmo código (set -e).
func shc(script string, env ...string) {
	var cmd *exec.Cmd
	if _, err := exec.LookPath("bash"); err == nil {
		cmd = exec.Command("bash", "-c", "set -o pipefail; "+script)
	} else {
		cmd = exec.Command("sh", "-c", script)
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
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
	case "debian", "ubuntu", "linuxmint", "zorin":
		logInfo("System detected: " + osRelease("PRETTY_NAME"))
	default:
		logError("Unsupported system: " + id)
		os.Exit(1)
	}
}

func installDependencies() {
	logInfo("Installing base dependencies...")
	run("apt", "update", "-y")
	run("apt", "install", "-y",
		"ca-certificates",
		"curl",
		"gnupg",
		"lsb-release")
}

func enableOpentofuRepo() {
	if !arquivo(opentofuKeyring) {
		logInfo("Importing official OpenTofu GPG key...")
		shc(`curl -fsSL https://packages.opentofu.org/opentofu/tofu/gpgkey | gpg --dearmor -o "$OPENTOFU_KEYRING"`,
			"OPENTOFU_KEYRING="+opentofuKeyring)
	} else {
		logInfo("OpenTofu GPG key already installed.")
	}

	if !arquivo(opentofuList) {
		logInfo("Adding official OpenTofu repository...")
		shc(`echo "deb [signed-by=$OPENTOFU_KEYRING] https://packages.opentofu.org/opentofu/tofu/apt $(lsb_release -cs) main" > "$OPENTOFU_LIST"`,
			"OPENTOFU_KEYRING="+opentofuKeyring,
			"OPENTOFU_LIST="+opentofuList)
	} else {
		logInfo("OpenTofu repository already configured.")
	}
}

func installOpentofu() {
	logInfo("Updating indices and installing OpenTofu (LTS)...")
	run("apt", "update", "-y")
	run("apt", "install", "-y", "tofu")
}

func verifyInstallation() {
	logInfo("Verifying OpenTofu installation...")

	if _, err := exec.LookPath("tofu"); err != nil {
		logError("OpenTofu was not installed correctly.")
		os.Exit(1)
	}

	fmt.Println("========================================")
	logInfo("OpenTofu installed successfully")
	fmt.Println("========================================")
	run("tofu", "version")
	fmt.Println("========================================")
}

func main() {
	prepararSaida()
	checkRoot()
	checkCommands()
	detectOS()
	installDependencies()
	enableOpentofuRepo()
	installOpentofu()
	verifyInstallation()
}

// --- fim do arquivo polyglot (tofudeb.go/tofudeb.sh) ---

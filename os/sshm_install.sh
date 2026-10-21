/* 2>/dev/null
# ---------------------------------------------------------------------------
#  sshm_install.sh — polyglot: este mesmo arquivo é sshm_install.go (Go) e
#  sshm_install.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run sshm_install.go [argumentos]
#    Bash ...: bash sshm_install.sh  [argumentos]   (exige a toolchain Go)
#
#  Projeto: GETDEV
#  Script: sshm_install.sh
#  Description: Installation script for SSHM (rsd-sshm) - A modern SSH Manager
#  Author: ranlens.denck
#  Category: os
#  Compatibility: Ubuntu, Debian, CentOS, Fedora, Arch, macOS, Windows
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[sshm_install.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[sshm_install.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-sshm_install"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/sshm_install.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init sshm_install >/dev/null 2>&1) || :

_bin="$_trab/sshm_install"
if [ ! -x "$_bin" ] || [ "$_trab/sshm_install.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando sshm_install.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o sshm_install.tmp sshm_install.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o sshm_install.tmp sshm_install.go) ||
			_die "falha ao compilar $_trab/sshm_install.go"
	}
	mv -f "$_trab/sshm_install.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (sshm_install.go/sshm_install.sh) ---'
*/
// sshm_install.go — instalador do SSHM (rsd-sshm), um gerenciador SSH moderno.
//
// Porta fiel do script original sshm_install.sh (GETDEV): SEM root check
// (fiel — o original também não tem), detecta o sistema com "uname -s"
// (Linux/Darwin seguem; qualquer outro SO é recusado com mensagem indicando
// PowerShell no Windows), roda o pipeline oficial verbatim
// "curl -sSL https://raw.githubusercontent.com/Gu1llaum-3/sshm/main/install/unix.sh | bash"
// com pipefail e termina sugerindo o comando "sshm".
//
// Execução:
//
//	go run sshm_install.go
//	bash sshm_install.sh    (mesmo arquivo, polyglot Go + sh)
package main

import (
	"bytes"
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

func logInfo(msg string)  { fmt.Println(pintar(cVerde, "[INFO]") + " " + msg) }
func logError(msg string) { fmt.Println(pintar(cVerm, "[ERROR]") + " " + msg) }

// ---------------------------------------------------------------------------
// Execução (equivalente ao set -euo pipefail do original; o IFS=$'\n\t' não
// tem equivalente em Go — não há splitting de palavras a reproduzir)
// ---------------------------------------------------------------------------

// shc roda um pipeline verbatim do original com pipefail (bash; sh como
// fallback) e a saída em tempo real. Falha não tratada encerra o programa com
// o mesmo código (set -e).
func shc(script string) {
	var cmd *exec.Cmd
	if _, err := exec.LookPath("bash"); err == nil {
		cmd = exec.Command("bash", "-c", "set -o pipefail; "+script)
	} else {
		cmd = exec.Command("sh", "-c", script)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
}

// saida devolve a saída do comando sem as quebras de linha finais (command
// substitution, como em OS_TYPE="$(uname -s)"). Falha não tratada encerra o
// programa com o mesmo código (set -e).
func saida(nome string, args ...string) string {
	cmd := exec.Command(nome, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
	return buf.String()
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

// ---------------------------------------------------------------------------
// Instalação
// ---------------------------------------------------------------------------

func installSSHM() {
	logInfo("Starting SSHM installation...")

	// Detect OS
	osType := strings.TrimRight(saida("uname", "-s"), "\n")

	switch {
	case strings.HasPrefix(osType, "Linux"), strings.HasPrefix(osType, "Darwin"):
		logInfo("Installing for Unix/Linux/macOS...")
		shc(`curl -sSL https://raw.githubusercontent.com/Gu1llaum-3/sshm/main/install/unix.sh | bash`)
	default:
		logError("Unsupported OS: " + osType + ". For Windows, use PowerShell instructions.")
		os.Exit(1)
	}

	logInfo("SSHM installation process triggered. Use 'sshm' to launch the TUI.")
}

func main() {
	prepararSaida()
	installSSHM()
}

// --- fim do arquivo polyglot (sshm_install.go/sshm_install.sh) ---

/* 2>/dev/null
# ---------------------------------------------------------------------------
#  black.sh — polyglot: este mesmo arquivo é black.go (Go) e black.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run black.go [argumentos]
#    Bash ...: bash black.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      black.sh
#  Description: Black Team Reconnaissance - Deep Offensive Discovery
#  Author:      GETDEV Team
#  Category:    Security / Black Team
#  Compatibility: Linux systems (Debian/RHEL)
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[black.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[black.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-black"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/black.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init black >/dev/null 2>&1) || :

_bin="$_trab/black"
if [ ! -x "$_bin" ] || [ "$_trab/black.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando black.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o black.tmp black.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o black.tmp black.go) ||
			_die "falha ao compilar $_trab/black.go"
	}
	mv -f "$_trab/black.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (black.go/black.sh) ---'
*/
// black.go — reconhecimento ofensivo da Black Team (Black Team Reconnaissance).
//
// Porta fiel do script original black_team.sh (GETDEV): levantamento profundo
// de sistema (kernel, distribuição, uptime, usuários com shell de login),
// de rede (interfaces IPv4 e portas TCP escutando lidas de /proc/net/tcp),
// caça a credenciais expostas (~/.ssh, histories, /etc/shadow, /root/.ssh)
// e teste dos caminhos de exfiltração via ping. Nenhum texto de saída foi
// traduzido ou alterado.
//
// Fidelidade ao original (bug mantido de propósito): em recon_network o
// printf "%d" recebe "0xlocal_address" (linha de cabeçalho de /proc/net/tcp,
// inválida como hexadecimal), falha e, sob o set -e do original, aborta o
// script ANTES de hunt_credentials e check_exfiltration — o porte aborta no
// mesmo ponto, com mensagem de erro equivalente e exit 1.
//
// Execução:
//
//	go run black.go
//	bash black.sh          (mesmo arquivo, polyglot Go + sh)
package main

import (
	"bytes"
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
	cGray  = "\033[1;30m"
	cRed   = "\033[0;31m"
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
	fmt.Println(pintar(cGray, "[INFO]") + " " + fmt.Sprintf(format, a...))
}

func logError(format string, a ...interface{}) {
	fmt.Println(pintar(cRed, "[ERROR]") + " " + fmt.Sprintf(format, a...))
}

func logBlack(format string, a ...interface{}) {
	fmt.Println(pintar(cGray, "[BLACK]") + " " + fmt.Sprintf(format, a...))
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

// shc roda um trecho de shell verbatim do original (pipelines, redirecionamentos).
func shc(script string) {
	run("sh", "-c", script)
}

// cap roda `sh -c <script>` devolvendo o stdout, fiel a $(...) do original:
// o stderr passa direto e a falha NÃO aborta (o original descarta o status).
func cap(script string) string {
	cmd := exec.Command("sh", "-c", script)
	cmd.Stderr = os.Stderr
	var b bytes.Buffer
	cmd.Stdout = &b
	_ = cmd.Run()
	return strings.TrimRight(b.String(), "\n")
}

// morrer reproduz a saída do set -e: sai com o código do comando que falhou.
func morrer(err error) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// ---------------------------------------------------------------------------
// Reconhecimento
// ---------------------------------------------------------------------------

func header() {
	fmt.Println(pintar(cGray, "===================================================="))
	fmt.Println(pintar(cGray, "          BLACK TEAM OFFENSIVE RECON                "))
	fmt.Println(pintar(cGray, "===================================================="))
}

func reconSystem() {
	logBlack("[+] Deep System Reconnaissance")
	logInfo("Kernel: %s", cap("uname -a"))
	logInfo("Distribution: %s", cap(`cat /etc/os-release | grep PRETTY_NAME | cut -d= -f2 | tr -d '"'`))
	logInfo("Uptime: %s", cap("uptime -p"))

	logInfo("Users with Login Shell:")
	shc(`grep -E "/bin/bash|/bin/sh|/bin/zsh" /etc/passwd | cut -d: -f1 | xargs echo`)
}

func reconNetwork() {
	logBlack("[+] Deep Network Reconnaissance")
	logInfo("Local Interfaces & Neighbors:")
	shc(`ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | xargs echo`)

	logInfo("Listening Services (Raw TCP):")
	// Bug fiel do original: "0xlocal_address" (cabeçalho de /proc/net/tcp) não é
	// um número hexadecimal válido; o printf falha e o script aborta aqui.
	shc(`cat /proc/net/tcp | awk '{print $2}' | cut -d: -f2 | sort | uniq | while read -r port; do printf "  - Port: %d\n" "0x$port"; done`)
}

func huntCredentials() {
	logBlack("[+] Hunting for Exposed Credentials")
	home, err := os.UserHomeDir()
	if err != nil {
		morrer(err)
	}
	searchPaths := []string{
		home + "/.ssh",
		home + "/.bash_history",
		home + "/.zsh_history",
		"/etc/shadow",
		"/root/.ssh",
	}

	for _, p := range searchPaths {
		if legivel(p) {
			logError("VULNERABILITY: Path '%s' is READABLE by current user!", p)
		} else {
			logInfo("Path '%s' is protected (Not readable).", p)
		}
	}
}

// legivel equivale ao [[ -r "$p" ]] do original.
func legivel(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func checkExfiltration() {
	logBlack("[+] Testing Data Exfiltration Paths")
	targets := []string{"8.8.8.8", "google.com", "github.com"}

	for _, t := range targets {
		if pingOK(t) {
			logError("Exfiltration to %s is POSSIBLE.", t)
		} else {
			logInfo("Exfiltration to %s is BLOCKED.", t)
		}
	}
}

// pingOK espelha `ping -c 1 -W 2 "$t" &>/dev/null` (sem ping = falso, como no if).
func pingOK(t string) bool {
	cmd := exec.Command("ping", "-c", "1", "-W", "2", t)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run() == nil
}

func main() {
	prepararSaida()
	header()
	reconSystem()
	reconNetwork()
	huntCredentials()
	checkExfiltration()
	fmt.Println()
	fmt.Println(pintar(cGray, "===================================================="))
	logInfo("Black Team Recon Complete.")
	fmt.Println(pintar(cGray, "===================================================="))
}

// --- fim do arquivo polyglot (black.go/black.sh) ---

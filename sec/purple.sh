/* 2>/dev/null
# ---------------------------------------------------------------------------
#  purple.sh — polyglot: este mesmo arquivo é purple.go (Go) e purple.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run purple.go [argumentos]
#    Bash ...: bash purple.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      purple.sh
#  Description: Purple Team Orchestrator - Real Attack & Defense Exercises
#  Author:      GETDEV Team
#  Category:    Security / Purple Team
#  Compatibility: Linux systems (Debian/RHEL)
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[purple.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[purple.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-purple"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/purple.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init purple >/dev/null 2>&1) || :

_bin="$_trab/purple"
if [ ! -x "$_bin" ] || [ "$_trab/purple.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando purple.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o purple.tmp purple.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o purple.tmp purple.go) ||
			_die "falha ao compilar $_trab/purple.go"
	}
	mv -f "$_trab/purple.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (purple.go/purple.sh) ---'
*/
// purple.go — orquestrador de exercícios Purple Team (Purple Team Orchestrator).
//
// Porta fiel do script original purple_team.sh (GETDEV): exige root, roda a
// auditoria da Red Team (bash ./red.sh, dependente do diretório corrente),
// pausa ("Press Enter"), roda o hardening da Blue Team (bash ./blue.sh),
// pausa de novo e verifica os resultados (arquivo de sysctl e
// "PermitRootLogin no" no sshd_config, com PASS/FAIL). Nenhum texto de saída
// foi traduzido ou alterado.
//
// Bug fiel do original: no ramo FAIL da checagem do SSH o script chama
// log_warn, função INEXISTENTE no purple — sob o set -e isso aborta com
// "command not found". O porte reproduz exatamente isso: escreve
// "log_warn: command not found" em stderr e sai com 1 naquele ponto.
//
// Execução:
//
//	go run purple.go
//	bash purple.sh         (mesmo arquivo, polyglot Go + sh)
package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// ---------------------------------------------------------------------------
// Saída (mesmas cores e labels do original; cor só em TTY e sem NO_COLOR)
// ---------------------------------------------------------------------------

var corAtiva bool

const (
	cReset  = "\033[0m"
	cGreen  = "\033[0;32m"
	cRed    = "\033[0;31m"
	cPurple = "\033[0;35m"
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
	fmt.Println(pintar(cGreen, "[INFO]") + " " + fmt.Sprintf(format, a...))
}

func logError(format string, a ...interface{}) {
	fmt.Println(pintar(cRed, "[ERROR]") + " " + fmt.Sprintf(format, a...))
}

func logPurple(format string, a ...interface{}) {
	fmt.Println(pintar(cPurple, "[PURPLE]") + " " + fmt.Sprintf(format, a...))
}

// ---------------------------------------------------------------------------
// Execução (equivalente ao set -euo pipefail do original)
// ---------------------------------------------------------------------------

// leitor compartilhado do stdin para as pausas "Press Enter".
var leitor = bufio.NewReader(os.Stdin)

// run executa um comando com stdin/stdout/stderr em tempo real. Falha não
// tratada encerra o programa com o mesmo código de saída do comando.
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
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// ehArquivo equivale a [[ -f "$1" ]].
func ehArquivo(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// pausar equivale a `echo -e "\n--- Press Enter ... ---"; read -r`.
// EOF no stdin cancela (exit 1), como falharia o read sob set -e.
func pausar(mensagem string) {
	fmt.Println()
	fmt.Println(mensagem)
	if _, err := leitor.ReadString('\n'); err != nil {
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Orquestração
// ---------------------------------------------------------------------------

func checkRoot() {
	if os.Geteuid() != 0 {
		logError("This script must be run as root.")
		os.Exit(1)
	}
}

func runRedActivity() {
	logPurple("Step 1: Running Red Team Audit (Offensive)...")
	if ehArquivo("./red.sh") {
		run("bash", "./red.sh")
	} else {
		logError("red.sh not found in current directory.")
	}
}

func runBlueActivity() {
	logPurple("Step 2: Running Blue Team Hardening (Defensive)...")
	if ehArquivo("./blue.sh") {
		run("bash", "./blue.sh")
	} else {
		logError("blue.sh not found in current directory.")
	}
}

func verifyResults() {
	logPurple("Step 3: Verifying Security Improvements...")
	logInfo("Checking if kernel hardening was applied...")
	if ehArquivo("/etc/sysctl.d/99-getdev-hardening.conf") {
		logInfo("Kernel hardening config found. (PASS)")
	} else {
		logError("Kernel hardening config NOT found. (FAIL)")
	}

	logInfo("Checking SSH root login status...")
	grep := exec.Command("grep", "-q", "^PermitRootLogin no", "/etc/ssh/sshd_config")
	if grep.Run() == nil {
		logInfo("SSH Root Login is disabled. (PASS)")
	} else {
		// Bug fiel: o original chama log_warn, que não existe neste script.
		// Sob set -e isso aborta com "command not found"; aqui, idem, exit 1.
		fmt.Fprintln(os.Stderr, "log_warn: command not found")
		os.Exit(1)
	}
}

func banner() {
	fmt.Println(pintar(cPurple, "===================================================="))
	fmt.Println(pintar(cPurple, "       PURPLE TEAM SECURITY ORCHESTRATOR           "))
	fmt.Println(pintar(cPurple, "===================================================="))
}

func main() {
	prepararSaida()
	checkRoot()
	banner()

	runRedActivity()
	pausar("--- Press Enter to proceed to Defense Phase ---")

	runBlueActivity()
	pausar("--- Press Enter to proceed to Verification Phase ---")

	verifyResults()

	fmt.Println(pintar(cPurple, "===================================================="))
	fmt.Println(pintar(cPurple, "       PURPLE TEAM EXERCISE COMPLETED              "))
	fmt.Println(pintar(cPurple, "===================================================="))
}

// --- fim do arquivo polyglot (purple.go/purple.sh) ---

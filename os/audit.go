/* 2>/dev/null
# ---------------------------------------------------------------------------
#  audit.sh — polyglot: este mesmo arquivo é audit.go (Go) e audit.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run audit.go [argumentos]
#    Bash ...: bash audit.sh  [argumentos]   (exige a toolchain Go)
#
#  Projeto: GETDEV
#  Script: audit.sh
#  Description: Defensive Audit Tool for Linux
#  Author: ranlens.denck
#  Category: os
#  Compatibility: Ubuntu, Debian, RHEL, Rocky, Alma, CentOS, Arch
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[audit.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[audit.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-audit"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/audit.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init audit >/dev/null 2>&1) || :

_bin="$_trab/audit"
if [ ! -x "$_bin" ] || [ "$_trab/audit.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando audit.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o audit.tmp audit.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o audit.tmp audit.go) ||
			_die "falha ao compilar $_trab/audit.go"
	}
	mv -f "$_trab/audit.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (audit.go/audit.sh) ---'
*/
// audit.go — defensive audit tool for Linux (porte fiel de audit.sh).
//
// Porta fiel do script audit.sh (GETDEV). Com --show executa a análise
// defensiva: persistência do journal, presença do auditd, logins remotos de
// root nas últimas 24h, top de tentativas de brute force, usuários com
// UID >= 1000, comandos recentes de sudo e portas suspeitas (4444, 5555 e
// 1337). Com --apply ativa o journal persistente. --dry-run apenas anuncia
// que nada será aplicado. Nenhuma mensagem de saída foi traduzida ou
// alterada.
//
// Execução:
//
//	go run audit.go [--show|--dry-run|--apply|--help]
//	bash audit.sh  [--show|--dry-run|--apply|--help]
package main

import (
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
	cReset   = "\033[0m"
	cVerde   = "\033[0;32m"
	cVerm    = "\033[0;31m"
	cAmarelo = "\033[1;33m"
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

func logWarn(format string, a ...interface{}) {
	fmt.Println(pintar(cAmarelo, "[WARN]") + " " + fmt.Sprintf(format, a...))
}

// log_err do original escreve no stdout (echo), não em stderr.
func logErr(format string, a ...interface{}) {
	fmt.Println(pintar(cVerm, "[ERROR]") + " " + fmt.Sprintf(format, a...))
}

// ---------------------------------------------------------------------------
// Execução (equivalente ao set -euo pipefail do original)
// ---------------------------------------------------------------------------

// run executa um comando com a saída em tempo real. Falha não tratada encerra
// o programa com código 1 — o próprio comando já imprimiu a mensagem de erro,
// como no set -euo pipefail do original.
func run(nome string, args ...string) {
	cmd := exec.Command(nome, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			logErr("%v", err)
		}
		os.Exit(1)
	}
}

// shc roda um trecho de shell verbatim do original (pipelines).
func shc(script string) {
	run("sh", "-c", script)
}

// shcTest roda um trecho de shell verbatim tratando o status como condição
// (equivalente a `if ! cmd` e a `cmd || fallback`): a falha não encerra o
// programa, só devolve false.
func shcTest(script string) bool {
	cmd := exec.Command("sh", "-c", script)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run() == nil
}

// ok testa um comando com a saída descartada (guards do original: `&>/dev/null`,
// `>/dev/null` e `-q`). A falha vira false, nunca exit.
func ok(nome string, args ...string) bool {
	return exec.Command(nome, args...).Run() == nil
}

// runIgnored executa um comando ignorando a falha (o original usa `|| true`).
func runIgnored(nome string, args ...string) {
	cmd := exec.Command(nome, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_ = cmd.Run()
}

// ---------------------------------------------------------------------------
// Help e privilégios
// ---------------------------------------------------------------------------

func printHelp() {
	fmt.Print(`Usage: audit.sh [--show|--dry-run|--apply|--help]

--show     : Inventory + Defensive analysis
--dry-run  : Show defense actions (does not apply)
--apply    : Activate auditing and persistent logs
--help     : This help
`)
}

// requireRoot: checagem exatamente nos caminhos --show e --apply, como no
// original (EUID != 0 → mensagem e exit 1).
func requireRoot() {
	if os.Geteuid() != 0 {
		logErr("This script must be run as root")
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Checagens do --show
// ---------------------------------------------------------------------------

func journalPersistent() {
	logInfo("Checking persistent journal...")
	if !ok("grep", "-q", "^Storage=persistent", "/etc/systemd/journald.conf") {
		logWarn("Journal is NOT persistent")
		logInfo("Suggestion: Set Storage=persistent in /etc/systemd/journald.conf")
	} else {
		logInfo("Journal is persistent.")
	}
}

func checkAuditd() {
	if _, err := exec.LookPath("auditctl"); err != nil {
		logWarn("auditctl NOT installed")
		logInfo("Suggestion: Install auditd package")
	} else {
		logInfo("auditd is installed.")
	}
}

func rootRemoteLogin() {
	logInfo("Checking for remote root logins in the last 24h...")
	if !shcTest(`journalctl _COMM=sshd --since "24 hours ago" | grep -i "Accepted" | grep -q "root"`) {
		logWarn("ROOT logged in remotely!")
	} else {
		logInfo("No remote root logins detected.")
	}
}

func bruteforce() {
	logInfo("Top failed login attempts in the last 24h:")
	shc(`journalctl _COMM=sshd --since "24 hours ago" | grep -i "Failed password" | awk '{print $(NF-3)}' | sort | uniq -c | sort -nr | head`)
}

func recentUsers() {
	logInfo("Users with UID >= 1000:")
	run("awk", "-F:", `$3>=1000 {print $1, $6}`, "/etc/passwd")
}

func sudoCommands() {
	logInfo("Sudo commands in the last 24h:")
	run("journalctl", "_COMM=sudo", "--since", "24 hours ago")
}

func suspiciousProcess() {
	logInfo("Checking for suspicious ports (4444, 5555, 1337)...")
	if !shcTest(`ss -tupan 2>/dev/null | grep -E ":(4444|5555|1337)"`) {
		logInfo("No suspicious ports found.")
	}
}

// ---------------------------------------------------------------------------
// Execução
// ---------------------------------------------------------------------------

func main() {
	prepararSaida()

	// Original: `if [[ $# -eq 0 ]]; then print_help; exit 0; fi`
	if len(os.Args) == 1 {
		printHelp()
		os.Exit(0)
	}

	// Original: `case "$1" in` — apenas o primeiro argumento é inspecionado.
	switch os.Args[1] {
	case "--show":
		requireRoot()
		journalPersistent()
		checkAuditd()
		rootRemoteLogin()
		bruteforce()
		recentUsers()
		sudoCommands()
		suspiciousProcess()
	case "--dry-run":
		logInfo("DRY RUN: No changes will be applied.")
	case "--apply":
		requireRoot()
		logInfo("Applying persistent journal configuration...")
		runIgnored("sed", "-i", "s/^#Storage=auto/Storage=persistent/", "/etc/systemd/journald.conf")
		run("systemctl", "restart", "systemd-journald")
		logInfo("Done.")
	case "--help":
		printHelp()
	default:
		printHelp()
		os.Exit(1)
	}
}

// --- fim do arquivo polyglot (audit.go/audit.sh) ---

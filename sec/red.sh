/* 2>/dev/null
# ---------------------------------------------------------------------------
#  red.sh — polyglot: este mesmo arquivo é red.go (Go) e red.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run red.go [argumentos]
#    Bash ...: bash red.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      red.sh
#  Description: Red Team Security Auditor - Offensive reconnaissance and audit
#  Author:      GETDEV Team
#  Category:    Security / Red Team
#  Compatibility: Linux systems (Debian/RHEL/Arch)
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[red.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[red.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-red"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/red.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init red >/dev/null 2>&1) || :

_bin="$_trab/red"
if [ ! -x "$_bin" ] || [ "$_trab/red.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando red.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o red.tmp red.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o red.tmp red.go) ||
			_die "falha ao compilar $_trab/red.go"
	}
	mv -f "$_trab/red.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (red.go/red.sh) ---'
*/
// red.go — auditor de segurança da Red Team (Red Team Security Auditor).
//
// Porta fiel do script original red_team.sh (GETDEV): auditoria ofensiva
// somente leitura — reconhecimento de rede (interfaces IPv4, portas via
// ss/netstat com fallback em /proc/net/tcp), descoberta de binários SUID
// (com fallback -perm -4000), busca de arquivos sensíveis (.ssh, histories,
// .env, wp-config.php, .git, .aws, config.php) em /home e /var/www,
// capabilities perigosas (getcap -r), processos root escutando (uid:0) e
// diretórios graváveis no PATH. Root é opcional: sem ele o script apenas
// avisa. Nenhum texto de saída foi traduzido ou alterado.
//
// Execução:
//
//	go run red.go
//	bash red.sh            (mesmo arquivo, polyglot Go + sh)
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// ---------------------------------------------------------------------------
// Saída (mesmas cores e labels do original; cor só em TTY e sem NO_COLOR)
// ---------------------------------------------------------------------------

var corAtiva bool

const (
	cReset  = "\033[0m"
	cRed    = "\033[0;31m"
	cYellow = "\033[1;33m"
	cGreen  = "\033[0;32m"
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

func logWarn(format string, a ...interface{}) {
	fmt.Println(pintar(cYellow, "[WARN]") + " " + fmt.Sprintf(format, a...))
}

func logCrit(format string, a ...interface{}) {
	fmt.Println(pintar(cRed, "[CRIT]") + " " + fmt.Sprintf(format, a...))
}

// secao imprime o cabeçalho de seção: echo -e "\n${RED}[+] ..."
func secao(txt string) {
	fmt.Println()
	fmt.Println(pintar(cRed, txt))
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

// shc roda um pipeline verbatim do original com a saída em tempo real.
func shc(script string) {
	run("sh", "-c", script)
}

// capOK roda `sh -c <script>` devolvendo o stdout e o status (stderr passa
// direto). É a base de cap (status descartado, como em $(...)) e capDie
// (falha aborta, como sob set -e + pipefail).
func capOK(script string) (string, error) {
	cmd := exec.Command("sh", "-c", script)
	cmd.Stderr = os.Stderr
	var b bytes.Buffer
	cmd.Stdout = &b
	err := cmd.Run()
	return strings.TrimRight(b.String(), "\n"), err
}

// cap roda um pipeline devolvendo o stdout sem abortar, fiel a $(...) mascarado.
func cap(script string) string {
	out, _ := capOK(script)
	return out
}

// capDie roda um pipeline e aborta se ele falhar (equivalente a find|while,
// getcap|while etc. sob pipefail: o status é o do comando que pode falhar).
func capDie(script string) string {
	out, err := capOK(script)
	if err != nil {
		morrer(err)
	}
	return out
}

// linhas devolve as linhas não vazias de uma saída de comando.
func linhas(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(s, "\n")
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

// temBinário equivale a command -v do original.
func temBinario(nome string) bool {
	_, err := exec.LookPath(nome)
	return err == nil
}

// ---------------------------------------------------------------------------
// Auditoria
// ---------------------------------------------------------------------------

func header() {
	fmt.Println(pintar(cRed, "===================================================="))
	fmt.Println(pintar(cRed, "          RED TEAM SECURITY AUDITOR                 "))
	fmt.Println(pintar(cRed, "===================================================="))
}

func checkRoot() {
	if os.Geteuid() != 0 {
		logWarn("Running without root. Some checks will be skipped.")
	}
}

func scanNetwork() {
	secao("[+] Network Reconnaissance")
	logInfo("Active Interfaces & IPs:")
	ips := capDie(`ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}'`)
	for _, ip := range linhas(ips) {
		logInfo(" Found IP: %s", ip)
	}

	logInfo("Open Ports (Local):")
	switch {
	case temBinario("ss"):
		shc("ss -tulpn | grep LISTEN")
	case temBinario("netstat"):
		shc("netstat -tulpn | grep LISTEN")
	default:
		logWarn("Neither 'ss' nor 'netstat' found. Checking /proc/net/tcp...")
		shc(`cat /proc/net/tcp | awk '{print $2}' | cut -d: -f2 | sort | uniq`)
	}
}

func huntSuid() {
	secao("[+] SUID/SGID Binary Discovery")
	logInfo("Searching for binaries with SUID/SGID bit...")
	suidFiles := cap(`find /usr/bin /usr/sbin /bin /sbin -perm /4000 -type f 2>/dev/null || find /usr/bin /usr/sbin /bin /sbin -perm -4000 -type f 2>/dev/null`)

	if suidFiles != "" {
		for _, file := range linhas(suidFiles) {
			logWarn(" Found SUID: %s", file)
		}
	} else {
		logInfo(" No suspicious SUID binaries found.")
	}
}

func checkCapabilities() {
	secao("[+] Binary Capabilities Check")
	if temBinario("getcap") {
		logInfo("Checking for dangerous capabilities...")
		out := capDie(`getcap -r /usr/bin /usr/sbin /bin /sbin 2>/dev/null`)
		for _, line := range linhas(out) {
			logWarn(" Found Cap: %s", line)
		}
	} else {
		logWarn(" 'getcap' command not found. Skipping.")
	}
}

func auditSensitiveFiles() {
	secao("[+] Sensitive File Discovery")
	patterns := []string{".ssh", ".bash_history", ".env", "config.php", "wp-config.php", ".git", ".aws"}

	for _, pattern := range patterns {
		logInfo("Searching for '%s' in /home and /var/www...", pattern)
		out := capDie(fmt.Sprintf(`find /home /var/www -name %q -type f,d 2>/dev/null`, pattern))
		for _, found := range linhas(out) {
			logCrit(" FOUND SENSITIVE: %s", found)
		}
	}
}

func auditProcesses() {
	secao("[+] Process & Permission Audit")
	logInfo("Root processes with network listening:")
	if temBinario("ss") {
		out, err := capOK(`ss -tulpn | grep "uid:0"`)
		if err != nil {
			logInfo(" None found.")
		} else if out != "" {
			fmt.Println(out)
		}
	}

	logInfo("World-writable directories in PATH:")
	for _, dir := range strings.Split(os.Getenv("PATH"), ":") {
		fi, err := os.Stat(dir)
		if err != nil || !fi.IsDir() {
			continue
		}
		// [[ -w "$dir" ]] do original usa access(2) — para root vale para
		// praticamente todo diretório, fiel ao comportamento original.
		const wOK = 2 // access(2): W_OK (o syscall do Linux não exporta a constante)
		writable := syscall.Access(dir, wOK) == nil
		if writable && fi.Mode()&os.ModeSticky != 0 {
			logWarn(" Sticky bit set on %s (OK)", dir)
		} else if writable {
			logCrit(" WORLD WRITABLE DIR IN PATH: %s", dir)
		}
	}
}

func main() {
	prepararSaida()
	header()
	checkRoot()
	scanNetwork()
	huntSuid()
	checkCapabilities()
	auditSensitiveFiles()
	auditProcesses()
	fmt.Println()
	fmt.Println(pintar(cRed, "===================================================="))
	logInfo("Audit Complete. Review findings carefully.")
	fmt.Println(pintar(cRed, "===================================================="))
}

// --- fim do arquivo polyglot (red.go/red.sh) ---

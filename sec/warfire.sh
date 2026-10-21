/* 2>/dev/null
# ---------------------------------------------------------------------------
#  warfire.sh — polyglot: este mesmo arquivo é warfire.go (Go) e warfire.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run warfire.go [argumentos]
#    Bash ...: bash warfire.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      warfire.sh
#  Description: ULTIMATE DEFENDER - Emergency Incident Response & Isolation
#  Author:      GETDEV Team
#  Category:    Security / Emergency
#  Compatibility: Linux (Debian, RHEL, Arch) - Requires root and 'chattr'
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[warfire.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[warfire.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-warfire"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/warfire.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init warfire >/dev/null 2>&1) || :

_bin="$_trab/warfire"
if [ ! -x "$_bin" ] || [ "$_trab/warfire.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando warfire.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o warfire.tmp warfire.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o warfire.tmp warfire.go) ||
			_die "falha ao compilar $_trab/warfire.go"
	}
	mv -f "$_trab/warfire.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (warfire.go/warfire.sh) ---'
*/
// warfire.go — resposta a incidente de emergência (ULTIMATE DEFENDER).
//
// Porta fiel do script original warfire.sh (GETDEV). ATENÇÃO: é um script de
// resposta a incidente com efeitos REAIS no sistema — exige root, não pede
// confirmação e não suaviza nada. $1 (opcional) é o SAFE_IP: o único endereço
// que NÃO é bloqueado. O roteiro, na ordem do original:
//
//  1. AIX: comparação /proc vs ps (processos ocultos), greps em lsmod por
//     padrões maliciosos e checagem de logs zerados nos últimos 10 minutos.
//  2. Imutabilidade: chattr +i em /usr/bin/{ps,ls,netstat,ss,top,chattr},
//     chattr +a em todos os arquivos de /var/log e criação do vault
//     /var/log/warfire_vault (append-only).
//  3. Forense: coleta para /tmp/.warfire_evidence_<ts> (processos, rede,
//     lsof, módulos, logins, history, arquivos modificados) + tar.gz.
//  4. Isolamento: pkill -9 -t das outras sessões pts, iptables -A INPUT -s
//     <ip> -j DROP por sessão remota, chattr +i em /etc/{passwd,shadow,
//     group,gshadow,sudoers}, iptables -P INPUT DROP aceitando apenas
//     ESTABLISHED,RELATED (e a porta 22 do SAFE_IP, se informado).
//  5. Kernel: sysctl kernel.modules_disabled=1, kernel.dmesg_restrict=1 e
//     kernel.kexec_load_disabled=1.
//
// Nenhum texto de saída foi traduzido ou alterado.
//
// Execução:
//
//	go run warfire.go [SAFE_IP]
//	bash warfire.sh  [SAFE_IP]   (mesmo arquivo, polyglot Go + sh)
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Saída (mesmas cores e labels do original; cor só em TTY e sem NO_COLOR)
// ---------------------------------------------------------------------------

var corAtiva bool

const (
	cReset   = "\033[0m"
	cBoldRed = "\033[1;31m"
	cYellow  = "\033[1;33m"
	cGreen   = "\033[0;32m"
	cBlue    = "\033[0;34m"
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

func logCrit(format string, a ...interface{}) {
	fmt.Println(pintar(cBoldRed, "[CRITICAL]") + " " + fmt.Sprintf(format, a...))
}

func logWarn(format string, a ...interface{}) {
	fmt.Println(pintar(cYellow, "[WARNING]") + " " + fmt.Sprintf(format, a...))
}

func logInfo(format string, a ...interface{}) {
	fmt.Println(pintar(cGreen, "[INFO]") + " " + fmt.Sprintf(format, a...))
}

func logAix(format string, a ...interface{}) {
	fmt.Println(pintar(cBlue, "[AIX-ENGINE]") + " " + fmt.Sprintf(format, a...))
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

// runD equivale a `cmd 2>/dev/null`: stderr descartado e falha fatal (tar).
func runD(nome string, args ...string) {
	cmd := exec.Command(nome, args...)
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
}

// runI equivale a `cmd || true`: falha tolerada, saída em tempo real.
func runI(nome string, args ...string) {
	cmd := exec.Command(nome, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = cmd.Run()
}

// runQI equivale a `cmd 2>/dev/null || true`: stderr descartado e falha tolerada.
func runQI(nome string, args ...string) {
	cmd := exec.Command(nome, args...)
	_ = cmd.Run()
}

// redirDie equivale a `cmd ... > arquivo` com saída em arquivo e falha fatal.
func redirDie(destino, nome string, args ...string) {
	f, err := os.Create(destino)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cmd := exec.Command(nome, args...)
	cmd.Stdout, cmd.Stderr = f, os.Stderr
	rerr := cmd.Run()
	f.Close()
	if rerr != nil {
		morrer(rerr)
	}
}

// shc roda um trecho de shell verbatim do original com saída em tempo real.
func shc(script string) {
	run("sh", "-c", script)
}

// shcOK equivale a `grep ... && log_crit ... || true`: nunca aborta e o
// status diz se o corpo do && deve rodar.
func shcOK(script string) bool {
	cmd := exec.Command("sh", "-c", script)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run() == nil
}

// ok equivale a `if cmd; then ...`: testa o status sem abortar.
func ok(nome string, args ...string) bool {
	cmd := exec.Command(nome, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run() == nil
}

// capOK roda `sh -c <script>` devolvendo o stdout e o status (stderr passa
// direto). É a base de cap (status descartado, como em $(...)/local x=$(...))
// e capDie (falha aborta, como sob set -e + pipefail).
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

// capDie roda um pipeline e aborta se ele falhar (equivalente a find|while e
// who|grep pts sob pipefail: o status vem do comando que pode falhar).
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

// ---------------------------------------------------------------------------
// Globais (avaliadas no início do script original, antes do main)
// ---------------------------------------------------------------------------

var (
	safeIP      string // $1 opcional: IP que NÃO será bloqueado
	evidenceDir string // /tmp/.warfire_evidence_<ts>
	logVault    string // /var/log/warfire_vault
)

// ---------------------------------------------------------------------------
// Fases
// ---------------------------------------------------------------------------

func header() {
	run("clear")
	fmt.Println(pintar(cBoldRed, "####################################################"))
	fmt.Println(pintar(cBoldRed, "#                                                  #"))
	fmt.Println(pintar(cBoldRed, "#             WARFIRE: ULTIMATE DEFENDER           #"))
	fmt.Println(pintar(cBoldRed, "#         EMERGENCY INCIDENT RESPONSE ACTIVE       #"))
	fmt.Println(pintar(cBoldRed, "#                                                  #"))
	fmt.Println(pintar(cBoldRed, "####################################################"))
}

func checkRoot() {
	if os.Geteuid() != 0 {
		fmt.Println("ERROR: Warfire MUST run as root to combat root-level threats.")
		os.Exit(1)
	}
}

// 1. AIX (Automated Intelligence X-Ray) - Detect Anomalias
func aixAnalyze() {
	logAix("Initializing X-Ray Scan...")

	// Detect hidden processes (ps vs /proc)
	logAix("Checking for hidden processes...")
	procCount := atoiOuZero(cap(`ls /proc | grep -E '^[0-9]+$' | wc -l`))
	psCount := atoiOuZero(cap(`ps -e --no-headers | wc -l`))
	if procCount > psCount+20 {
		logCrit("AIX ALERT: Hidden processes detected! Potential Rootkit activity.")
	}

	// Detect unauthorized kernel modules
	logAix("Checking for suspicious kernel modules...")
	shc(`lsmod | awk '{print $1}' > /tmp/.modules`)
	// Compare with a clean state if available, or just look for common malicious names
	if shcOK(`grep -E "hacker|root|hide|backdoor" /tmp/.modules`) {
		logCrit("AIX ALERT: Malicious module pattern found!")
	}

	// Detect log deletion activity
	logAix("Scanning for truncated or deleted logs...")
	out := capDie(`find /var/log -type f -size 0 -mmin -10`)
	for _, lg := range linhas(out) {
		logWarn("AIX ALERT: Log %s was recently truncated/wiped!", lg)
	}
}

// atoiOuZero converte a saída de wc -l; valor não numérico não dispara alerta.
func atoiOuZero(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// 2. Immutability & Protection - Lock the gates
func protectAssets() {
	logInfo("Enforcing System Immutability...")

	// Protect critical binaries from being modified/replaced
	binaries := []string{"/usr/bin/ps", "/usr/bin/ls", "/usr/bin/netstat", "/usr/bin/ss", "/usr/bin/top", "/usr/bin/chattr"}
	for _, bin := range binaries {
		if fi, err := os.Stat(bin); err == nil && fi.Mode().IsRegular() {
			runQI("chattr", "+i", bin)
		}
	}

	// Protect existing logs from deletion
	logInfo("Locking /var/log/ files (Append-only mode)...")
	shcOK(`find /var/log -type f -exec chattr +a {} + 2>/dev/null`)

	// Create emergency vault
	run("mkdir", "-p", logVault)
	run("chattr", "+a", logVault)
}

// 3. Isolation Mode - Trap the Attacker
func isolateAttacker() {
	logCrit("INITIATING ISOLATION PROTOCOL...")

	// Identify current shell PID to avoid self-kill
	// (o original declara `local my_pid=$$` e nunca o usa — removido no porte)
	mySSHIP := cap(`who am i | awk '{print $5}' | tr -d '()'`)

	// Kill all other SSH sessions
	logWarn("Terminating all unauthorized SSH sessions...")
	out := capDie(`who | grep pts`)
	for _, line := range linhas(out) {
		campos := strings.Fields(line)
		pts, ip := "", ""
		if len(campos) > 1 {
			pts = campos[1]
		}
		if len(campos) > 4 {
			ip = strings.Trim(campos[4], "()")
		}

		if ip != mySSHIP && ip != safeIP {
			logCrit("Kicking attacker from %s (%s)...", pts, ip)
			runI("pkill", "-9", "-t", pts)
			// Block the IP immediately
			run("iptables", "-A", "INPUT", "-s", ip, "-j", "DROP")
		}
	}

	// Lock password database and sensitive files
	logInfo("Locking accounts and credential databases...")
	runQI("chattr", "+i", "/etc/passwd", "/etc/shadow", "/etc/group", "/etc/gshadow", "/etc/sudoers")

	// Disable new network connections except established
	logInfo("Locking down network (Established connections only)...")
	run("iptables", "-P", "INPUT", "DROP")
	run("iptables", "-A", "INPUT", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT")
	if safeIP != "" {
		run("iptables", "-A", "INPUT", "-s", safeIP, "-p", "tcp", "--dport", "22", "-j", "ACCEPT")
	}
}

// 4. Evidence Extraction - Save everything
func exportEvidence() {
	logInfo("Collecting forensic evidence to %s...", evidenceDir)
	run("mkdir", "-p", evidenceDir)

	redirDie(evidenceDir+"/process_tree.txt", "ps", "auxwf")
	redirDie(evidenceDir+"/network_listen.txt", "netstat", "-tulpn")
	redirDie(evidenceDir+"/open_files_net.txt", "lsof", "-i")
	redirDie(evidenceDir+"/kernel_modules.txt", "lsmod")
	redirDie(evidenceDir+"/last_logins.txt", "last", "-n", "50")
	// `history > arquivo || true`: builtin do bash não-interativo gera saída vazia.
	_ = os.WriteFile(evidenceDir+"/root_history.txt", nil, 0666)

	// Export all commands typed in the last 30 mins
	logInfo("Snapshot of modified files in /etc and /var/www...")
	redirDie(evidenceDir+"/modified_files.txt", "find", "/etc", "/var/www", "-type", "f", "-mmin", "-30")

	runD("tar", "-czf", evidenceDir+".tar.gz", evidenceDir)
	logInfo("Forensic package created: %s.tar.gz", evidenceDir)
}

// 5. Kernel Lockdown
func kernelLockdown() {
	logWarn("Hardening Kernel against further tampering...")
	// Prevent loading new modules
	if !ok("sysctl", "-w", "kernel.modules_disabled=1") {
		logWarn("Failed to disable module loading (already disabled or not supported).")
	}
	// Prevent dmesg access to attackers
	run("sysctl", "-w", "kernel.dmesg_restrict=1")
	// Disable kexec (prevent kernel replacement)
	runI("sysctl", "-w", "kernel.kexec_load_disabled=1")
}

func main() {
	prepararSaida()
	if len(os.Args) > 1 {
		safeIP = os.Args[1] // Optional: IP to NOT block
	}
	timestamp := capDie(`date +%Y%m%d_%H%M%S`)
	evidenceDir = "/tmp/.warfire_evidence_" + timestamp
	logVault = "/var/log/warfire_vault"

	header()
	checkRoot()

	logCrit("PHASE 1: AIX DETECTION")
	aixAnalyze()

	logCrit("PHASE 2: ASSET PROTECTION")
	protectAssets()

	logCrit("PHASE 3: FORENSICS")
	exportEvidence()

	logCrit("PHASE 4: ISOLATION & LOCKDOWN")
	isolateAttacker()
	kernelLockdown()

	fmt.Println()
	fmt.Println(pintar(cGreen, "####################################################"))
	logInfo("WARFIRE DEFENSE SYSTEM IS FULLY ENGAGED.")
	logInfo("The attacker has been isolated and system is locked.")
	logInfo("Review forensics at: %s.tar.gz", evidenceDir)
	fmt.Println(pintar(cGreen, "####################################################"))
}

// --- fim do arquivo polyglot (warfire.go/warfire.sh) ---

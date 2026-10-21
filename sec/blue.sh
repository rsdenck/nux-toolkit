/* 2>/dev/null
# ---------------------------------------------------------------------------
#  blue.sh — polyglot: este mesmo arquivo é blue.go (Go) e blue.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run blue.go [argumentos]
#    Bash ...: bash blue.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      blue.sh
#  Description: Blue Team Defense & Hardening - Secure the system
#  Author:      GETDEV Team
#  Category:    Security / Blue Team
#  Compatibility: Linux systems (Debian/RHEL)
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[blue.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[blue.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-blue"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/blue.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init blue >/dev/null 2>&1) || :

_bin="$_trab/blue"
if [ ! -x "$_bin" ] || [ "$_trab/blue.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando blue.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o blue.tmp blue.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o blue.tmp blue.go) ||
			_die "falha ao compilar $_trab/blue.go"
	}
	mv -f "$_trab/blue.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (blue.go/blue.sh) ---'
*/
// blue.go — defesa e endurecimento do sistema (Blue Team Defense & Hardening).
//
// Porta fiel do script original blue_team.sh (GETDEV): exige root, faz
// backup do /etc/ssh/sshd_config com timestamp e aplica PermitRootLogin no
// e PermitEmptyPasswords no, reinicia o serviço ssh/sshd, configura o
// firewall em cadeia (UFW, senão Firewalld, senão iptables), grava o
// /etc/sysctl.d/99-getdev-hardening.conf e aplica com sysctl -p, e por fim
// só AVISA sobre auditd e fail2ban. Sem nenhuma confirmação — fiel ao
// original. Nenhum texto de saída foi traduzido ou alterado.
//
// Execução:
//
//	go run blue.go
//	bash blue.sh           (mesmo arquivo, polyglot Go + sh)
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
	cBlue  = "\033[0;34m"
	cGreen = "\033[0;32m"
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
	fmt.Println(pintar(cGreen, "[INFO]") + " " + fmt.Sprintf(format, a...))
}

func logWarn(format string, a ...interface{}) {
	fmt.Println(pintar(cBlue, "[BLUE]") + " " + fmt.Sprintf(format, a...))
}

func logError(format string, a ...interface{}) {
	fmt.Println(pintar(cRed, "[ERROR]") + " " + fmt.Sprintf(format, a...))
}

// secaoAzul imprime o cabeçalho de seção: echo -e "\n${BLUE}[+] ..."
func secaoAzul(txt string) {
	fmt.Println()
	fmt.Println(pintar(cBlue, txt))
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

// shc roda um comando verbatim do original (seds) com a saída em tempo real.
func shc(script string) {
	run("sh", "-c", script)
}

// runQuietOK equivale a `cmd &>/dev/null || true`: saída descartada e falha
// tolerada (usado pelo sysctl -p do hardening de kernel).
func runQuietOK(nome string, args ...string) {
	cmd := exec.Command(nome, args...)
	_ = cmd.Run()
}

// ok equivale a `if cmd; then ...`/`command -v`: testa o status sem abortar.
func ok(nome string, args ...string) bool {
	cmd := exec.Command(nome, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run() == nil
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

// temBinário equivale a command -v do original.
func temBinario(nome string) bool {
	_, err := exec.LookPath(nome)
	return err == nil
}

// ---------------------------------------------------------------------------
// Hardening
// ---------------------------------------------------------------------------

func header() {
	fmt.Println(pintar(cBlue, "===================================================="))
	fmt.Println(pintar(cBlue, "          BLUE TEAM DEFENSE & HARDENING             "))
	fmt.Println(pintar(cBlue, "===================================================="))
}

func checkRoot() {
	if os.Geteuid() != 0 {
		logError("This script must be run as root.")
		os.Exit(1)
	}
}

func hardenSSH() {
	secaoAzul("[+] SSH Hardening")
	sshConfig := "/etc/ssh/sshd_config"
	if fi, err := os.Stat(sshConfig); err == nil && !fi.IsDir() {
		logInfo("Backing up SSH config...")
		ts := cap("date +%F_%T")
		run("cp", sshConfig, sshConfig+".bak_"+ts)

		logInfo("Applying security settings...")
		// Disable Root Login, Disable Empty Passwords, Use Protocol 2
		shc("sed -i 's/^#PermitRootLogin.*/PermitRootLogin no/' " + sshConfig)
		shc("sed -i 's/^PermitRootLogin.*/PermitRootLogin no/' " + sshConfig)
		shc("sed -i 's/^#PermitEmptyPasswords.*/PermitEmptyPasswords no/' " + sshConfig)
		shc("sed -i 's/^PermitEmptyPasswords.*/PermitEmptyPasswords no/' " + sshConfig)

		if ok("systemctl", "is-active", "--quiet", "ssh") {
			logInfo("Restarting SSH service...")
			run("systemctl", "restart", "ssh")
		} else if ok("systemctl", "is-active", "--quiet", "sshd") {
			logInfo("Restarting sshd service...")
			run("systemctl", "restart", "sshd")
		}
	} else {
		logError("SSH config not found at %s", sshConfig)
	}
}

func setupFirewall() {
	secaoAzul("[+] Firewall Configuration")
	switch {
	case temBinario("ufw"):
		logInfo("Detected UFW. Configuring...")
		run("ufw", "default", "deny", "incoming")
		run("ufw", "default", "allow", "outgoing")
		run("ufw", "allow", "ssh")
		run("ufw", "--force", "enable")
	case temBinario("firewalld"):
		logInfo("Detected Firewalld. Configuring...")
		run("systemctl", "start", "firewalld")
		run("systemctl", "enable", "firewalld")
		run("firewall-cmd", "--permanent", "--add-service=ssh")
		run("firewall-cmd", "--reload")
	default:
		logWarn("No high-level firewall tool (UFW/Firewalld) found. Using iptables basics...")
		run("iptables", "-A", "INPUT", "-m", "conntrack", "--ctstate", "ESTABLISHED,RELATED", "-j", "ACCEPT")
		run("iptables", "-A", "INPUT", "-p", "tcp", "--dport", "22", "-j", "ACCEPT")
		run("iptables", "-A", "INPUT", "-i", "lo", "-j", "ACCEPT")
		run("iptables", "-P", "INPUT", "DROP")
	}
}

func hardenKernel() {
	secaoAzul("[+] Kernel Hardening (sysctl)")
	sysctlConf := "/etc/sysctl.d/99-getdev-hardening.conf"
	logInfo("Creating kernel hardening config...")
	conteudo := `# GETDEV Blue Team Hardening
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.default.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.default.send_redirects = 0
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.default.accept_source_route = 0
net.ipv4.icmp_echo_ignore_broadcasts = 1
net.ipv4.conf.all.rp_filter = 1
net.ipv4.conf.default.rp_filter = 1
kernel.randomize_va_space = 2
`
	if err := os.WriteFile(sysctlConf, []byte(conteudo), 0666); err != nil {
		morrer(err)
	}
	runQuietOK("sysctl", "-p", sysctlConf)
}

func checkMonitoring() {
	secaoAzul("[+] Security Monitoring Status")

	// Auditd
	if ok("systemctl", "is-active", "--quiet", "auditd") {
		logInfo("Auditd is running.")
	} else {
		logWarn("Auditd is not running. Consider installing it.")
	}

	// Fail2Ban
	if temBinario("fail2ban-client") {
		logInfo("Fail2Ban is installed.")
	} else {
		logWarn("Fail2Ban is NOT installed. Highly recommended for SSH protection.")
	}
}

func main() {
	prepararSaida()
	header()
	checkRoot()
	hardenSSH()
	setupFirewall()
	hardenKernel()
	checkMonitoring()
	fmt.Println()
	fmt.Println(pintar(cBlue, "===================================================="))
	logInfo("Hardening Complete. System is now more secure.")
	fmt.Println(pintar(cBlue, "===================================================="))
}

// --- fim do arquivo polyglot (blue.go/blue.sh) ---

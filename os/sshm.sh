/* 2>/dev/null
# ---------------------------------------------------------------------------
#  sshm.sh — polyglot: este mesmo arquivo é sshm.go (Go) e sshm.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run sshm.go [argumentos]
#    Bash ...: bash sshm.sh  [argumentos]   (exige a toolchain Go)
#
#  Projeto: GETDEV
#  Script: sshm.sh
#  Description: SSH Management and Hardening (Port, Root, Auditd, New User)
#  Author: ranlens.denck
#  Category: os
#  Compatibility: Ubuntu, Debian, CentOS, Fedora, Arch
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[sshm.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[sshm.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-sshm"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/sshm.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init sshm >/dev/null 2>&1) || :

_bin="$_trab/sshm"
if [ ! -x "$_bin" ] || [ "$_trab/sshm.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando sshm.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o sshm.tmp sshm.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o sshm.tmp sshm.go) ||
			_die "falha ao compilar $_trab/sshm.go"
	}
	mv -f "$_trab/sshm.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (sshm.go/sshm.sh) ---'
*/
// sshm.go — SSH management and hardening (porte fiel de sshm.sh).
//
// Porta fiel do script sshm.sh (GETDEV): instala o auditd conforme a distro,
// cria o usuário pedido com sudo NOPASSWD e aplica o hardening do sshd_config
// (porta nova, PermitRootLogin no, PasswordAuthentication no, AllowUsers),
// com backup do arquivo antes das mudanças e restart do serviço. Quirks
// fiéis mantidos: a flag --apply é parseada mas nunca usada (as mudanças
// sempre são aplicadas) e o serviço reiniciado é sempre "sshd". Nenhuma
// mensagem de saída foi traduzida ou alterada.
//
// Execução:
//
//	go run sshm.go --user <username> [--port <port>] [--apply]
//	bash sshm.sh  --user <username> [--port <port>] [--apply]
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Estado (variáveis globais do original)
// ---------------------------------------------------------------------------

var (
	// SSH_PORT_NEW=4022 (default; sobrescrito por --port).
	sshPortNew = "4022"
	// USER_TO_ADD="" (obrigatório via --user).
	userToAdd = ""
	// APPLY=true do original: a flag --apply é parseada mas NUNCA é usada —
	// as mudanças são sempre aplicadas (quirk fiel mantido; é variável de
	// pacote porque local não usada não compila em Go).
	apply = false
	// DISTRO do detect_distro do original.
	distro string
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

// log_warn é definida no original mas nunca chamada; mantida por fidelidade.
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

// shc roda um trecho de shell verbatim do original (`&&`, `||`, pipelines).
func shc(script string) {
	run("sh", "-c", script)
}

// ok testa um comando com a saída descartada (guards do original com
// `&>/dev/null` e `-q`). A falha vira false, nunca exit.
func ok(nome string, args ...string) bool {
	return exec.Command(nome, args...).Run() == nil
}

// ---------------------------------------------------------------------------
// Help e privilégios
// ---------------------------------------------------------------------------

func usage() {
	// Original: echo "Usage: $0 ..." (auto-referência usa o nome novo).
	fmt.Println("Usage: sshm.sh --user <username> [--port <port>] [--apply]")
	os.Exit(1)
}

// check_root do original usa `id -u` (não $EUID).
func checkRoot() {
	out, err := exec.Command("id", "-u").Output()
	if err != nil {
		logErr("%v", err)
		os.Exit(1)
	}
	uid, conv := strconv.Atoi(strings.TrimSpace(string(out)))
	if conv != nil || uid != 0 {
		logErr("This script must be run as root.")
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// Distro e auditd
// ---------------------------------------------------------------------------

// detect_distro: lê o ID do /etc/os-release; sem arquivo → "unknown".
func detectDistro() {
	distro = "unknown"
	dados, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return
	}
	for _, ln := range strings.Split(string(dados), "\n") {
		if strings.HasPrefix(strings.TrimSpace(ln), "ID=") {
			distro = strings.Trim(strings.TrimPrefix(strings.TrimSpace(ln), "ID="), `"'`)
			return
		}
	}
}

func installAuditd() {
	logInfo("Checking auditd...")
	// Original: `command -v auditd &>/dev/null` (verifica "auditd", não "auditctl").
	if _, err := exec.LookPath("auditd"); err != nil {
		logInfo("Installing auditd...")
		// case "$DISTRO" do original, sem default (distro desconhecida não
		// instala nada e segue direto para o systemctl enable --now auditd).
		switch distro {
		case "ubuntu", "debian":
			shc("apt-get update && apt-get install -y auditd")
		case "centos", "rhel", "fedora":
			shc("dnf install -y audit || yum install -y audit")
		case "arch":
			shc("pacman -S --noconfirm audit")
		}
		run("systemctl", "enable", "--now", "auditd")
	} else {
		logInfo("Auditd already installed.")
	}
}

// ---------------------------------------------------------------------------
// Usuário e sudo
// ---------------------------------------------------------------------------

func createUser() {
	if userToAdd == "" {
		logErr("User not defined. Use --user <name>")
		os.Exit(1)
	}

	if ok("id", userToAdd) {
		logInfo("User %s already exists.", userToAdd)
	} else {
		logInfo("Creating user %s...", userToAdd)
		run("useradd", "-m", "-s", "/bin/bash", userToAdd)
		logInfo("User created. Remember to set a password!")
	}

	logInfo("Configuring sudo for %s...", userToAdd)
	// Original: echo "$USER_TO_ADD ALL=(ALL) NOPASSWD:ALL" > "/etc/sudoers.d/$USER_TO_ADD"
	sudoers := "/etc/sudoers.d/" + userToAdd
	if err := os.WriteFile(sudoers, []byte(userToAdd+" ALL=(ALL) NOPASSWD:ALL\n"), 0o644); err != nil {
		logErr("%v", err)
		os.Exit(1)
	}
	run("chmod", "440", sudoers)
}

// ---------------------------------------------------------------------------
// Hardening do sshd_config
// ---------------------------------------------------------------------------

// appendLine reproduz `echo "..." >> arquivo` do original.
func appendLine(caminho, linha string) error {
	f, err := os.OpenFile(caminho, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(linha); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func configureSSH() {
	logInfo("Configuring SSH Hardening...")
	sshConfig := "/etc/ssh/sshd_config"
	// Original: cp "$SSH_CONFIG" "${SSH_CONFIG}.bak.$(date +%Y%m%d)"
	backup := sshConfig + ".bak." + time.Now().Format("20060102")
	run("cp", sshConfig, backup)

	// Change port (sed -i do original, verbatim)
	run("sed", "-i", "s/^#Port 22/Port "+sshPortNew+"/", sshConfig)
	run("sed", "-i", "s/^Port .*/Port "+sshPortNew+"/", sshConfig)

	// Disable Root and Password login (sed -i do original, verbatim)
	run("sed", "-i", `s/^#PermitRootLogin.*/PermitRootLogin no/`, sshConfig)
	run("sed", "-i", `s/^PermitRootLogin.*/PermitRootLogin no/`, sshConfig)
	run("sed", "-i", `s/^#PasswordAuthentication.*/PasswordAuthentication no/`, sshConfig)
	run("sed", "-i", `s/^PasswordAuthentication.*/PasswordAuthentication no/`, sshConfig)

	// AllowUsers
	if ok("grep", "-q", "^AllowUsers", sshConfig) {
		run("sed", "-i", "s/^AllowUsers.*/AllowUsers "+userToAdd+"/", sshConfig)
	} else {
		if err := appendLine(sshConfig, "AllowUsers "+userToAdd+"\n"); err != nil {
			logErr("%v", err)
			os.Exit(1)
		}
	}

	logInfo("SSH Config updated. Port: %s, User allowed: %s", sshPortNew, userToAdd)
}

func restartSSH() {
	logInfo("Restarting SSH service...")
	// Quirk fiel: o original reinicia sempre a unidade "sshd" (no Debian o
	// serviço se chama "ssh") — mantido como está.
	run("systemctl", "restart", "sshd")
}

// ---------------------------------------------------------------------------
// Parsing e execução
// ---------------------------------------------------------------------------

// unboundVar reproduz a mensagem do bash com `set -u` quando falta o valor de
// uma flag ($2 não definido): o original morre com exit 1.
func unboundVar() {
	fmt.Fprintln(os.Stderr, "sshm.sh: $2: unbound variable")
	os.Exit(1)
}

func main() {
	prepararSaida()

	// Parsing arguments (mesma ordem e mesmos guards do original)
	args := os.Args[1:]
	for i := 0; i < len(args); {
		switch args[i] {
		case "--user":
			if i+1 >= len(args) {
				unboundVar()
			}
			userToAdd = args[i+1]
			i += 2
		case "--port":
			if i+1 >= len(args) {
				unboundVar()
			}
			sshPortNew = args[i+1]
			i += 2
		case "--apply":
			apply = true // quirk fiel: parseada e nunca usada
			i++
		default:
			usage()
		}
	}

	if userToAdd == "" {
		usage()
	}

	checkRoot()
	detectDistro()
	installAuditd()
	createUser()
	configureSSH()
	restartSSH()

	logInfo("SSHM Hardening completed!")
}

// --- fim do arquivo polyglot (sshm.go/sshm.sh) ---

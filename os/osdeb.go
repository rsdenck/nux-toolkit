/* 2>/dev/null
# ---------------------------------------------------------------------------
#  osdeb.sh — polyglot: este mesmo arquivo é osdeb.go (Go) e osdeb.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run osdeb.go [argumentos]
#    Bash ...: bash osdeb.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      osdeb.sh
#  Description: Essential SysAdmin tools bootstrap for Debian-like systems
#  Author:      GETDEV Team
#  Category:    OS / Bootstrap
#  Compatibility: Debian, Ubuntu, ZorinOS, and derivatives
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[osdeb.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[osdeb.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-osdeb"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/osdeb.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init osdeb >/dev/null 2>&1) || :

_bin="$_trab/osdeb"
if [ ! -x "$_bin" ] || [ "$_trab/osdeb.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando osdeb.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o osdeb.tmp osdeb.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o osdeb.tmp osdeb.go) ||
			_die "falha ao compilar $_trab/osdeb.go"
	}
	mv -f "$_trab/osdeb.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (osdeb.go/osdeb.sh) ---'
*/
// osdeb.go — bootstrap de ferramentas essenciais de sysadmin em Debian-like.
//
// Porta fiel do script original osdeb.sh (GETDEV): exige root, espelha stdout
// e stderr em /var/log/osdeb-bootstrap.log (append) e no console, detecta a
// distro, habilita os repositórios adicionais (universe/multiverse/restricted
// em Ubuntu e derivados), instala os pacotes essenciais e finaliza com
// "apt autoremove -y" e "apt clean". Nenhum texto de saída foi traduzido.
//
// Execução:
//
//	go run osdeb.go
//	bash osdeb.sh          (mesmo arquivo, polyglot Go + sh)
package main

import (
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

func logInfo(msg string) {
	fmt.Fprintln(os.Stdout, pintar(cVerde, "[INFO]")+" "+msg)
}

func logError(msg string) {
	fmt.Fprintln(os.Stderr, pintar(cVerm, "[ERROR]")+" "+msg)
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

// runTrue executa um comando protegido por "|| true" no original: a falha é
// ignorada (a saída continua em tempo real).
func runTrue(nome string, args ...string) {
	cmd := exec.Command(nome, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = cmd.Run()
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

// espelharLog reproduz fielmente o "exec > >(tee -a LOG) 2>&1" do original:
// a partir daqui stdout e stderr vão para o console E, em append, para o
// arquivo de log — inclusive a saída dos subprocessos, que herdam os fds,
// exatamente como no script bash.
func espelharLog(logPath string) {
	r, w, err := os.Pipe()
	if err != nil {
		morrer(err)
	}
	tee := exec.Command("tee", "-a", logPath)
	tee.Stdin = r
	tee.Stdout = os.Stdout
	tee.Stderr = os.Stderr
	if err := tee.Start(); err != nil {
		morrer(err)
	}
	r.Close()
	os.Stdout = w
	os.Stderr = w
}

// ---------------------------------------------------------------------------
// Constantes e estado
// ---------------------------------------------------------------------------

const logPath = "/var/log/osdeb-bootstrap.log"

var (
	distroID   string
	distroLike string
)

// ---------------------------------------------------------------------------
// Aplicação
// ---------------------------------------------------------------------------

func main() {
	prepararSaida()

	// Check root
	if os.Geteuid() != 0 {
		logError("This script must be run as root")
		os.Exit(1)
	}

	espelharLog(logPath)

	logInfo("Starting Debian-like system bootstrap")
	// "Date: $(date)": no bash a falha do $(...) não aborta o script.
	if data, err := exec.Command("date").Output(); err == nil {
		logInfo("Date: " + strings.TrimSpace(string(data)))
	}

	detectDistro()
	enableRepos()
	installPackages()
	finalize()
}

// detectDistro espelha o "source /etc/os-release" do original (ID e ID_LIKE).
func detectDistro() {
	dados, err := os.ReadFile("/etc/os-release")
	if err != nil {
		logError("Could not detect distribution")
		os.Exit(1)
	}
	for _, ln := range strings.Split(string(dados), "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "ID=") {
			distroID = strings.Trim(strings.TrimPrefix(ln, "ID="), `"'`)
		} else if strings.HasPrefix(ln, "ID_LIKE=") {
			distroLike = strings.Trim(strings.TrimPrefix(ln, "ID_LIKE="), `"'`)
		}
	}
}

func enableRepos() {
	logInfo("Enabling additional repositories (if applicable)")

	run("apt", "update")
	run("apt", "install", "-y", "software-properties-common")

	// Ubuntu and derivatives
	if distroID == "ubuntu" || strings.Contains(distroLike, "ubuntu") {
		runTrue("add-apt-repository", "-y", "universe")
		runTrue("add-apt-repository", "-y", "multiverse")
		runTrue("add-apt-repository", "-y", "restricted")
	}

	run("apt", "update")
}

func installPackages() {
	logInfo("Installing essential packages")

	pkgs := []string{
		// Network / DNS
		"dnsutils", "iproute2", "net-tools", "nmap", "tcpdump", "traceroute", "mtr", "ethtool", "network-manager",
		// System / Debug
		"lsof", "strace", "htop", "iotop", "sysstat", "psmisc", "procps",
		// Security
		"fail2ban", "auditd", "ufw", "apparmor-utils",
		// Automation / DevOps
		"git", "curl", "wget", "jq", "rsync", "bash-completion",
		// Compression / Utilities
		"zip", "unzip", "tar", "gzip", "bzip2", "vim", "nano", "tree", "screen", "tmux",
	}

	run("apt", append([]string{"install", "-y"}, pkgs...)...)
}

func finalize() {
	logInfo("Finalizing configuration")
	run("apt", "autoremove", "-y")
	run("apt", "clean")
	logInfo("Bootstrap completed successfully")
}

// --- fim do arquivo polyglot (osdeb.go/osdeb.sh) ---

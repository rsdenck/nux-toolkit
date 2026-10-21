/* 2>/dev/null
# ---------------------------------------------------------------------------
#  postgrhel.sh — polyglot: este mesmo arquivo é postgrhel.go (Go) e postgrhel.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run postgrhel.go [argumentos]
#    Bash ...: bash postgrhel.sh [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      postgrhel.sh
#  Description: PostgreSQL 16 LTS Bootstrap for RHEL-like systems
#  Author:      GETDEV Team
#  Category:    Database
#  Compatibility: RHEL, Rocky Linux, AlmaLinux, CentOS
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[postgrhel.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[postgrhel.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-postgrhel"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/postgrhel.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init postgrhel >/dev/null 2>&1) || :

_bin="$_trab/postgrhel"
if [ ! -x "$_bin" ] || [ "$_trab/postgrhel.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando postgrhel.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o postgrhel.tmp postgrhel.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o postgrhel.tmp postgrhel.go) ||
			_die "falha ao compilar $_trab/postgrhel.go"
	}
	mv -f "$_trab/postgrhel.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (postgrhel.go/postgrhel.sh) ---'
*/
// postgrhel.go — bootstrap do PostgreSQL 16 LTS em RHEL-like.
//
// Porta fiel do script original postgrhel.sh (GETDEV): exige root, confere os
// comandos obrigatórios (dnf, rpm, curl), valida a distro pelo ID do
// /etc/os-release (rocky, almalinux, centos ou rhel), instala as dependências
// base, habilita o repositório oficial PGDG (guardado por rpm -q), desativa o
// módulo postgresql do sistema, instala o PostgreSQL 16 (server, client,
// contrib), inicializa o banco apenas se o data dir não existir, habilita e
// inicia o serviço e confere o resultado. Nenhum texto de saída foi traduzido.
//
// Execução:
//
//	go run postgrhel.go
//	bash postgrhel.sh       (mesmo arquivo, polyglot Go + sh)
package main

import (
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

// okExec corresponde aos guards "if cmd; then" / "cmd >/dev/null 2>&1" do
// original: devolve true quando o comando termina com código 0, sem ecoar saída.
func okExec(nome string, args ...string) bool {
	cmd := exec.Command(nome, args...)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run() == nil
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
// Globals (mesmos valores do original)
// ---------------------------------------------------------------------------

var (
	requiredCmds = []string{"dnf", "rpm", "curl"}
	prettyName   string
)

const (
	pgVersion = "16"
	pgService = "postgresql-" + pgVersion
	pgBin     = "/usr/pgsql-" + pgVersion + "/bin/psql"
	pgSetup   = "/usr/pgsql-" + pgVersion + "/bin/postgresql-" + pgVersion + "-setup"
)

// ---------------------------------------------------------------------------
// Aplicação
// ---------------------------------------------------------------------------

func main() {
	prepararSaida()

	checkRoot()
	checkCommands()
	detectOS()
	installDependencies()
	enablePGDGRepo()
	installPostgresql()
	initializeDatabase()
	enableService()
	verifyInstallation()

	logInfo("PostgreSQL " + pgVersion + " bootstrap completed successfully.")
}

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

// detectOS espelha o "source /etc/os-release" + case do original.
func detectOS() {
	dados, err := os.ReadFile("/etc/os-release")
	if err != nil {
		logError("Could not detect operating system.")
		os.Exit(1)
	}
	var osID string
	for _, ln := range strings.Split(string(dados), "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "ID=") {
			osID = strings.Trim(strings.TrimPrefix(ln, "ID="), `"'`)
		} else if strings.HasPrefix(ln, "PRETTY_NAME=") {
			prettyName = strings.Trim(strings.TrimPrefix(ln, "PRETTY_NAME="), `"'`)
		}
	}

	switch osID {
	case "rocky", "almalinux", "centos", "rhel":
		logInfo("System detected: " + prettyName)
	default:
		logError("Unsupported system: " + osID)
		os.Exit(1)
	}
}

func installDependencies() {
	logInfo("Installing base dependencies...")
	run("dnf", "install", "-y",
		"ca-certificates",
		"curl",
		"dnf-plugins-core")
}

func enablePGDGRepo() {
	if !okExec("rpm", "-q", "pgdg-redhat-repo") {
		logInfo("Installing official PostgreSQL repository (PGDG)...")
		// $(rpm -E %rhel): no bash a falha do $(...) não aborta o script.
		el, _ := exec.Command("rpm", "-E", "%rhel").Output()
		run("dnf", "install", "-y",
			"https://download.postgresql.org/pub/repos/yum/reporpms/EL-"+
				strings.TrimSpace(string(el))+
				"-x86_64/pgdg-redhat-repo-latest.noarch.rpm")
	} else {
		logInfo("PGDG repository already installed.")
	}

	logInfo("Disabling default system PostgreSQL module...")
	// Fidelidade ao original: SEM guard — uma falha do dnf aqui aborta o
	// bootstrap inteiro (set -e), como no script bash.
	run("dnf", "-qy", "module", "disable", "postgresql")
}

func installPostgresql() {
	logInfo("Installing PostgreSQL " + pgVersion + " (server, client, contrib)...")

	run("dnf", "install", "-y",
		"postgresql"+pgVersion+"-server",
		"postgresql"+pgVersion,
		"postgresql"+pgVersion+"-contrib")
}

func initializeDatabase() {
	if fi, err := os.Stat("/var/lib/pgsql/" + pgVersion + "/data"); err == nil && fi.IsDir() {
		logInfo("Database already initialized.")
		return
	}
	logInfo("Initializing database...")
	run(pgSetup, "initdb")
}

func enableService() {
	logInfo("Enabling and starting " + pgService + "...")
	run("systemctl", "enable", "--now", pgService)
}

func verifyInstallation() {
	logInfo("Verifying installation...")
	if !okExec("systemctl", "is-active", "--quiet", pgService) {
		logError("PostgreSQL failed to start.")
		os.Exit(1)
	}
	logInfo("PostgreSQL is running.")
	run(pgBin, "--version")
}

// --- fim do arquivo polyglot (postgrhel.go/postgrhel.sh) ---

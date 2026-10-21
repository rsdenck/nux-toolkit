/* 2>/dev/null
# ---------------------------------------------------------------------------
#  javarhel.sh — polyglot: este mesmo arquivo é javarhel.go (Go) e javarhel.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run javarhel.go [argumentos]
#    Bash ...: bash javarhel.sh  [argumentos]   (exige a toolchain Go)
#
#  Project: GETDEV
#  Script: javarhel.sh
#  Description: Java OpenJDK Bootstrap for RHEL-like systems
#  Author: ranlens.denck
#  Category: os
#  Compatibility: RHEL, Rocky, Alma, CentOS Stream
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[javarhel.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[javarhel.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-javarhel"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/javarhel.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init javarhel >/dev/null 2>&1) || :

_bin="$_trab/javarhel"
if [ ! -x "$_bin" ] || [ "$_trab/javarhel.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando javarhel.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o javarhel.tmp javarhel.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o javarhel.tmp javarhel.go) ||
			_die "falha ao compilar $_trab/javarhel.go"
	}
	mv -f "$_trab/javarhel.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (javarhel.go/javarhel.sh) ---'
*/
// javarhel.go — bootstrap do OpenJDK para sistemas RHEL-like.
//
// Porta fiel do script original javarhel.sh (GETDEV): exige root, redireciona
// stdout e stderr para /var/log/javarhel-bootstrap.log via tee (o tee do
// original vem DEPOIS do root check, e é mantido nessa ordem), detecta o
// sistema, garante os repositórios padrão (dnf install -y dnf-plugins-core e
// dnf makecache), instala java-11/17/21-openjdk apenas quando "dnf info" os
// encontra, define o padrão 21→17→11 com alternatives --set (caminhos sem
// sufixo de arquitetura, como no original) e imprime "java -version".
//
// Execução:
//
//	go run javarhel.go
//	bash javarhel.sh        (mesmo arquivo, polyglot Go + sh)
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
// Constantes (fiéis ao original)
// ---------------------------------------------------------------------------

const logPath = "/var/log/javarhel-bootstrap.log"

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

// morrer reproduz a saída do set -e: sai com o código do comando que falhou.
func morrer(err error) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	logError(err.Error())
	os.Exit(1)
}

// disponivel reproduz: comando ... &>/dev/null dentro de um if (sem set -e).
func disponivel(nome string, args ...string) bool {
	cmd := exec.Command(nome, args...)
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run() == nil
}

// ---------------------------------------------------------------------------
// Root e log (fiel: o exec tee vem DEPOIS do root check)
// ---------------------------------------------------------------------------

func checkRoot() {
	if os.Geteuid() != 0 {
		logError("This script must be run as root")
		os.Exit(1)
	}
}

// enviarParaLog reproduz: exec > >(tee -a "$LOG") 2>&1 — todo o resto do
// script (inclusive a saída dos comandos) passa pelo tee e vai para o log.
// As cores já foram decididas antes, porque o original usa constantes.
func enviarParaLog() {
	origOut, origErr := os.Stdout, os.Stderr
	pr, pw, err := os.Pipe()
	if err != nil {
		return
	}
	tee := exec.Command("tee", "-a", logPath)
	tee.Stdin = pr
	tee.Stdout = origOut
	tee.Stderr = origErr
	if err := tee.Start(); err != nil {
		pr.Close()
		pw.Close()
		return
	}
	pr.Close()
	os.Stdout = pw
	os.Stderr = pw
	go func() { _ = tee.Wait() }()
}

// ---------------------------------------------------------------------------
// Deteção de sistema
// ---------------------------------------------------------------------------

// osRelease lê uma variável de /etc/os-release (o ". /etc/os-release" original).
func osRelease(chave string) string {
	dados, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(dados), "\n") {
		ln = strings.TrimSpace(ln)
		if !strings.HasPrefix(ln, chave+"=") {
			continue
		}
		return strings.Trim(strings.TrimPrefix(ln, chave+"="), `"'`)
	}
	return ""
}

func detectOS() {
	if fi, err := os.Stat("/etc/os-release"); err != nil || !fi.Mode().IsRegular() {
		logError("Could not detect system")
		os.Exit(1)
	}
	logInfo(fmt.Sprintf("System detected: %s %s", osRelease("NAME"), osRelease("VERSION_ID")))
}

// ---------------------------------------------------------------------------
// Etapas do bootstrap (mesma ordem do original)
// ---------------------------------------------------------------------------

func enableRepos() {
	logInfo("Ensuring default repositories")
	run("dnf", "install", "-y", "dnf-plugins-core")
	run("dnf", "makecache")
}

func installJavaVersions() {
	logInfo("Installing available Java versions")

	javaPkgs := []string{
		"java-11-openjdk",
		"java-17-openjdk",
		"java-21-openjdk",
	}

	for _, pkg := range javaPkgs {
		if disponivel("dnf", "info", pkg) {
			logInfo("Installing " + pkg)
			run("dnf", "install", "-y", pkg)
		} else {
			logInfo("Package " + pkg + " not available in this system (skipped)")
		}
	}
}

// executavel reproduz o teste [[ -x arquivo ]].
func executavel(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0
}

func setLatestJavaDefault() {
	logInfo("Setting latest Java version as default")

	// Ordem e caminhos exatos do original (sem sufixo de arquitetura).
	candidates := []string{
		"/usr/lib/jvm/java-21-openjdk/bin/java",
		"/usr/lib/jvm/java-17-openjdk/bin/java",
		"/usr/lib/jvm/java-11-openjdk/bin/java",
	}

	var found []string
	for _, c := range candidates {
		if executavel(c) {
			found = append(found, c)
		}
	}

	if len(found) == 0 {
		logError("No Java versions found")
		os.Exit(1)
	}

	latestJava := found[0]

	run("alternatives", "--set", "java", latestJava)

	// "${LATEST_JAVA%/java}/javac" || true — melhor esforço, como no original.
	javac := latestJava
	if strings.HasSuffix(javac, "/java") {
		javac = strings.TrimSuffix(javac, "/java")
	}
	if err := exec.Command("alternatives", "--set", "javac", javac+"/javac").Run(); err != nil {
		// o original silencia a falha com || true
	}
}

func showJavaVersion() {
	logInfo("Current active Java version:")
	fmt.Println("---------------------------------------------")
	run("java", "-version")
	fmt.Println("---------------------------------------------")
}

func main() {
	prepararSaida()
	checkRoot()
	enviarParaLog()
	detectOS()
	enableRepos()
	installJavaVersions()
	setLatestJavaDefault()
	showJavaVersion()

	logInfo("Java RHEL-like bootstrap completed successfully!")
}

// --- fim do arquivo polyglot (javarhel.go/javarhel.sh) ---

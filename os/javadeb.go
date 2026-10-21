/* 2>/dev/null
# ---------------------------------------------------------------------------
#  javadeb.sh — polyglot: este mesmo arquivo é javadeb.go (Go) e javadeb.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run javadeb.go [argumentos]
#    Bash ...: bash javadeb.sh  [argumentos]   (exige a toolchain Go)
#
#  Project: GETDEV
#  Script: javadeb.sh
#  Description: Java OpenJDK Bootstrap for Debian-like systems
#  Author: ranlens.denck
#  Category: os
#  Compatibility: Debian, Ubuntu, ZorinOS, and derivatives
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[javadeb.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[javadeb.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-javadeb"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/javadeb.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init javadeb >/dev/null 2>&1) || :

_bin="$_trab/javadeb"
if [ ! -x "$_bin" ] || [ "$_trab/javadeb.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando javadeb.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o javadeb.tmp javadeb.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o javadeb.tmp javadeb.go) ||
			_die "falha ao compilar $_trab/javadeb.go"
	}
	mv -f "$_trab/javadeb.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (javadeb.go/javadeb.sh) ---'
*/
// javadeb.go — bootstrap do OpenJDK para sistemas Debian-like.
//
// Porta fiel do script original javadeb.sh (GETDEV): exige root, redireciona
// stdout e stderr para /var/log/javadeb-bootstrap.log via tee (o tee do
// original vem DEPOIS do root check, e é mantido nessa ordem), detecta o
// sistema, atualiza os índices com "apt update", instala openjdk-11/17/21-jdk
// apenas quando "apt-cache show" encontra o pacote, define o padrão 21→17→11
// com update-alternatives (caminhos hardcoded *-amd64, mantidos fielmente) e
// imprime "java -version".
//
// Execução:
//
//	go run javadeb.go
//	bash javadeb.sh        (mesmo arquivo, polyglot Go + sh)
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

const logPath = "/var/log/javadeb-bootstrap.log"

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

func updateRepos() {
	logInfo("Updating package indices")
	run("apt", "update")
}

func installJavaVersions() {
	logInfo("Installing available Java versions")

	javaPkgs := []string{
		"openjdk-11-jdk",
		"openjdk-17-jdk",
		"openjdk-21-jdk",
	}

	for _, pkg := range javaPkgs {
		if disponivel("apt-cache", "show", pkg) {
			logInfo("Installing " + pkg)
			run("apt", "install", "-y", pkg)
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

	// Ordem e caminhos exatos do original (hardcoded *-amd64, mantidos
	// fielmente: em arm64 nenhum caminho existe e o script falha, como no
	// original).
	candidates := []string{
		"/usr/lib/jvm/java-21-openjdk-amd64/bin/java",
		"/usr/lib/jvm/java-17-openjdk-amd64/bin/java",
		"/usr/lib/jvm/java-11-openjdk-amd64/bin/java",
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

	run("update-alternatives", "--set", "java", latestJava)

	// "${LATEST_JAVA%/java}/javac" || true — melhor esforço, como no original.
	javac := latestJava
	if strings.HasSuffix(javac, "/java") {
		javac = strings.TrimSuffix(javac, "/java")
	}
	if err := exec.Command("update-alternatives", "--set", "javac", javac+"/javac").Run(); err != nil {
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
	updateRepos()
	installJavaVersions()
	setLatestJavaDefault()
	showJavaVersion()

	logInfo("Java Debian-like bootstrap completed successfully!")
}

// --- fim do arquivo polyglot (javadeb.go/javadeb.sh) ---

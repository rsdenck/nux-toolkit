/* 2>/dev/null
# ---------------------------------------------------------------------------
#  gcloud.sh — polyglot: este mesmo arquivo é gcloud.go (Go) e gcloud.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run gcloud.go [argumentos]
#    Bash ...: bash gcloud.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      gcloud.sh
#  Description: Google Cloud SDK (gcloud CLI) Bootstrap
#  Author:      GETDEV Team
#  Category:    Cloud / Bootstrap
#  Compatibility: Ubuntu, Debian, CentOS, RHEL, Fedora, Arch
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[gcloud.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[gcloud.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-gcloud"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/gcloud.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init gcloud >/dev/null 2>&1) || :

_bin="$_trab/gcloud"
if [ ! -x "$_bin" ] || [ "$_trab/gcloud.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando gcloud.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o gcloud.tmp gcloud.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o gcloud.tmp gcloud.go) ||
			_die "falha ao compilar $_trab/gcloud.go"
	}
	mv -f "$_trab/gcloud.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (gcloud.go/gcloud.sh) ---'
*/
// gcloud.go — bootstrap do Google Cloud SDK (gcloud CLI).
//
// Porta fiel do script original gcloud.sh (GETDEV): detecta a distro via
// /etc/os-release e instala o gcloud CLI pelo ramo correspondente (apt/dnf/yum
// ou o instalador oficial via curl). Se o gcloud já estiver instalado, mostra
// a versão e sai. Nenhum texto de saída foi traduzido ou alterado.
//
// Execução:
//
//	go run gcloud.go
//	bash gcloud.sh        (mesmo arquivo, polyglot Go + sh)
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

func logInfo(format string, a ...interface{}) {
	fmt.Println(pintar(cVerde, "[INFO]") + " " + fmt.Sprintf(format, a...))
}

func logError(format string, a ...interface{}) {
	fmt.Println(pintar(cVerm, "[ERROR]") + " " + fmt.Sprintf(format, a...))
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

// shc roda um trecho de shell verbatim do original (pipelines, heredoc via tee).
func shc(script string) {
	run("sh", "-c", script)
}

// morrer reproduz a saída do set -e: sai com o código do comando que falhou.
func morrer(err error) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	logError("%v", err)
	os.Exit(1)
}

// ---------------------------------------------------------------------------
// Distro
// ---------------------------------------------------------------------------

var distro string

// detectDistro espelha o original: lê o ID do /etc/os-release ou "unknown".
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

// ---------------------------------------------------------------------------
// Instalação
// ---------------------------------------------------------------------------

func installGcloud() {
	logInfo("Installing Google Cloud SDK...")

	// guard de idempotência do original: já instalado → mostra versão e sai
	if _, err := exec.LookPath("gcloud"); err == nil {
		logInfo("gcloud CLI is already installed.")
		run("gcloud", "version")
		return
	}

	switch distro {
	case "ubuntu", "debian":
		shc("apt-get update && apt-get install -y apt-transport-https ca-certificates gnupg curl")
		shc("curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | gpg --dearmor -o /usr/share/keyrings/cloud.google.gpg")
		shc(`echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list`)
		shc("apt-get update && apt-get install -y google-cloud-cli")
	case "centos", "rhel", "fedora", "rocky", "almalinux":
		shc(`cat <<EOF | tee /etc/yum.repos.d/google-cloud-sdk.repo
[google-cloud-sdk]
name=Google Cloud SDK
baseurl=https://packages.cloud.google.com/yum/repos/cloud-sdk-el9-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=0
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF`)
		if _, err := exec.LookPath("dnf"); err == nil {
			run("dnf", "install", "-y", "google-cloud-cli")
		} else {
			run("yum", "install", "-y", "google-cloud-cli")
		}
	case "arch":
		logInfo("For Arch Linux, AUR (google-cloud-sdk) is recommended.")
		logInfo("Installing via official script as alternative...")
		shc("curl https://sdk.cloud.google.com | bash --disable-prompts")
	default:
		logInfo("Installing via official script...")
		shc("curl https://sdk.cloud.google.com | bash --disable-prompts")
	}

	logInfo("gcloud CLI installed successfully!")
}

func main() {
	prepararSaida()
	detectDistro()
	installGcloud()
}

// --- fim do arquivo polyglot (gcloud.go/gcloud.sh) ---

/* 2>/dev/null
# ---------------------------------------------------------------------------
#  extend_swap.sh — polyglot: este mesmo arquivo é extend_swap.go (Go) e extend_swap.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run extend_swap.go [tamanho_em_GB]
#    Bash ...: bash extend_swap.sh  [tamanho_em_GB]   (exige a toolchain Go)
#
#  Projeto: GETDEV
#  Script: extend_swap.sh
#  Description: Swap Manager (create, expand and validate)
#  Author: ranlens.denck
#  Category: os
#  Compatibility: All Linux systems
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[extend_swap.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[extend_swap.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-extend_swap"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/extend_swap.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init extend_swap >/dev/null 2>&1) || :

_bin="$_trab/extend_swap"
if [ ! -x "$_bin" ] || [ "$_trab/extend_swap.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando extend_swap.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o extend_swap.tmp extend_swap.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o extend_swap.tmp extend_swap.go) ||
			_die "falha ao compilar $_trab/extend_swap.go"
	}
	mv -f "$_trab/extend_swap.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (extend_swap.go/extend_swap.sh) ---'
*/
// extend_swap.go — gerenciador de swap (cria, ativa e persiste um swapfile).
//
// Porte fiel do script original extend_swap.sh (GETDEV): sem flags — o 1º
// argumento posicional é o tamanho em GB (padrão 4). Verifica os comandos
// exigidos, mostra `swapon --show` e `free -h` antes e depois, cria /swapfile
// com fallocate (fallback dd), chmod 600, mkswap, swapon e adiciona a linha
// ao /etc/fstab apenas se ela ainda não existir. Nenhum texto de saída foi
// traduzido ou alterado.
//
// Quirk fiel ao original (mantido de propósito): se /swapfile já existe, ele
// NÃO é redimensionado — create_swapfile apenas avisa e retorna, seguindo
// para ativação/persistência do arquivo existente.
//
// Execução:
//
//	go run extend_swap.go [tamanho_em_GB]
//	bash extend_swap.sh  [tamanho_em_GB]   (mesmo arquivo, polyglot Go + sh)
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

const (
	swapfile      = "/swapfile"
	defaultSizeGB = "4"
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

// morrer reproduz a saída do set -e: sai com o código do comando que falhou.
func morrer(err error) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	logError("%v", err)
	os.Exit(1)
}

// executar roda um comando com argv direto (equivale a `cmd "$arg"` do
// original); falha não tratada encerra o programa como faria set -e.
func executar(nome string, args ...string) {
	cmd := exec.Command(nome, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
}

// shEnv roda um trecho de shell verbatim do original (pipelines,
// redirecionamentos) com variáveis extras no ambiente.
func shEnv(script string, env ...string) error {
	cmd := exec.Command("sh", "-c", script)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// Funções do original
// ---------------------------------------------------------------------------

func checkRoot() {
	if os.Geteuid() != 0 {
		logError("This script must be run as root.")
		os.Exit(1)
	}
}

func checkCommands() {
	cmds := []string{"fallocate", "dd", "mkswap", "swapon", "swapoff", "free", "grep", "awk", "chmod", "chown"}
	for _, cmd := range cmds {
		if _, err := exec.LookPath(cmd); err != nil {
			logError("Required command not found: %s", cmd)
			os.Exit(1)
		}
	}
}

func showSwapStatus() {
	fmt.Println()
	fmt.Println(pintar(cVerde, "=========== CURRENT SWAP STATUS ==========="))
	// swapon --show || true
	cmd := exec.Command("swapon", "--show")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_ = cmd.Run()
	executar("free", "-h")
	fmt.Println(pintar(cVerde, "============================================"))
	fmt.Println()
}

func createSwapfile(sizeGB string) {
	if fi, err := os.Stat(swapfile); err == nil && !fi.IsDir() {
		// Fiel: arquivo já existente não é redimensionado.
		logInfo("Swapfile already exists at %s", swapfile)
		return
	}

	logInfo("Creating swapfile of %sGB...", sizeGB)

	// fallocate -l "${size_gb}G" "$SWAPFILE" 2>/dev/null
	fal := exec.Command("fallocate", "-l", sizeGB+"G", swapfile)
	fal.Stdout, fal.Stderr = os.Stdout, io.Discard
	if fal.Run() != nil {
		logInfo("fallocate not available, using dd (slower)...")
		executar("dd", "if=/dev/zero", "of="+swapfile, "bs=1G", "count="+sizeGB, "status=progress")
	}

	executar("chmod", "600", swapfile)
	executar("mkswap", swapfile)
}

func enableSwap() {
	// Pipeline original `swapon --show | grep -q "$SWAPFILE"` verbatim.
	if shEnv(`swapon --show | grep -q "$SWAPFILE"`, "SWAPFILE="+swapfile) == nil {
		logInfo("Swapfile is already active.")
		return
	}

	logInfo("Activating swapfile...")
	executar("swapon", swapfile)
}

func persistSwap() {
	if exec.Command("grep", "-q", "^"+swapfile, "/etc/fstab").Run() == nil {
		logInfo("Swapfile is already persistent in /etc/fstab")
		return
	}

	logInfo("Adding swapfile to /etc/fstab...")
	if err := shEnv(`echo "$SWAPFILE none swap sw 0 0" >> /etc/fstab`, "SWAPFILE="+swapfile); err != nil {
		morrer(err)
	}
}

// ---------------------------------------------------------------------------
// Aplicação (execução linear, como no original)
// ---------------------------------------------------------------------------

func main() {
	prepararSaida()

	// Execution
	checkRoot()
	checkCommands()
	showSwapStatus()
	sizeGB := defaultSizeGB
	if len(os.Args) > 1 {
		sizeGB = os.Args[1]
	}
	createSwapfile(sizeGB)
	enableSwap()
	persistSwap()
	showSwapStatus()

	logInfo("Swap configuration completed!")
}

// --- fim do arquivo polyglot (extend_swap.go/extend_swap.sh) ---

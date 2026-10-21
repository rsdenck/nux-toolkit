/* 2>/dev/null
# ---------------------------------------------------------------------------
#  ollama.sh — polyglot: este mesmo arquivo é ollama.go (Go) e ollama.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run ollama.go [argumentos]
#    Bash ...: bash ollama.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      ollama.sh
#  Description: Global Ollama and VS Code Continue extension configuration
#  Author:      GETDEV Team
#  Category:    AI / Automation
#  Compatibility: Linux (Debian, RHEL, etc.)
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[ollama.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[ollama.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-ollama"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/ollama.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init ollama >/dev/null 2>&1) || :

_bin="$_trab/ollama"
if [ ! -x "$_bin" ] || [ "$_trab/ollama.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando ollama.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o ollama.tmp ollama.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o ollama.tmp ollama.go) ||
			_die "falha ao compilar $_trab/ollama.go"
	}
	mv -f "$_trab/ollama.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (ollama.go/ollama.sh) ---'
*/
// ollama.go — configuração global do Ollama e da extensão Continue do VS Code.
//
// Porta fiel do script original ollama_vscode.sh (GETDEV): cria o diretório
// global ~/.continue, grava o config.yaml (idêntico ao original), sobe o
// serviço ollama em background se necessário, testa a API local, baixa os
// modelos requeridos e limpa o cache antigo do Continue.
//
// Execução:
//
//	go run ollama.go
//	bash ollama.sh        (mesmo arquivo, polyglot Go + sh)
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ---------------------------------------------------------------------------
// Config.yaml gravado em ~/.continue/config.yaml (idêntico ao heredoc original)
// ---------------------------------------------------------------------------

const continueConfig = `name: Global Ollama Continue Config
version: 1.0.0
schema: v1

models:
  - name: ollama-chat
    provider: ollama
    model: llama3.1:8b
    apiBase: http://localhost:11434
    roles:
      - chat
      - edit
      - apply

  - name: ollama-autocomplete
    provider: ollama
    model: qwen2.5-coder:1.5b-base
    apiBase: http://localhost:11434
    roles:
      - autocomplete

  - name: ollama-embed
    provider: ollama
    model: nomic-embed-text:latest
    apiBase: http://localhost:11434
    roles:
      - embed

context:
  providers:
    - type: code
    - type: diff
    - type: docs

allowAnonymousTelemetry: false
disableUpdateCheck: true
logLevel: INFO
`

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
// Serviço Ollama
// ---------------------------------------------------------------------------

// servidorRodando espelha: pgrep -x ollama >/dev/null
func servidorRodando() bool {
	return exec.Command("pgrep", "-x", "ollama").Run() == nil
}

// iniciarServico reproduz: nohup ollama serve >/tmp/ollama.log 2>&1 &
// (cmd.Start() sem Wait(), stdout e stderr em /tmp/ollama.log).
func iniciarServico() {
	f, err := os.OpenFile("/tmp/ollama.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		logError("%v", err)
		return
	}
	defer f.Close()
	cmd := exec.Command("ollama", "serve")
	cmd.Stdout, cmd.Stderr = f, f
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "nohup: failed to run command 'ollama': %v\n", err)
	}
}

func main() {
	prepararSaida()

	userHome := os.Getenv("HOME")
	continueDir := filepath.Join(userHome, ".continue")
	configFile := filepath.Join(continueDir, "config.yaml")

	logInfo("Creating global Continue directory...")
	if err := os.MkdirAll(continueDir, 0o755); err != nil {
		morrer(err)
	}

	logInfo("Generating global config.yaml...")
	if err := os.WriteFile(configFile, []byte(continueConfig), 0o644); err != nil {
		morrer(err)
	}

	logInfo("Ensuring correct permissions...")
	if err := os.Chmod(configFile, 0o600); err != nil {
		morrer(err)
	}

	logInfo("Checking Ollama status...")
	if !servidorRodando() {
		logInfo("Starting Ollama service...")
		iniciarServico()
		time.Sleep(3 * time.Second)
	}

	logInfo("Testing Ollama API...")
	if err := exec.Command("sh", "-c", `curl -s http://localhost:11434/api/tags | grep -q models`).Run(); err != nil {
		logError("Ollama API did not respond correctly")
		os.Exit(1)
	}

	logInfo("Pulling required models...")
	run("ollama", "pull", "llama3.1:8b")
	run("ollama", "pull", "qwen2.5-coder:1.5b-base")
	run("ollama", "pull", "nomic-embed-text")

	logInfo("Cleaning old Continue cache...")
	run("rm", "-rf",
		filepath.Join(continueDir, "index"),
		filepath.Join(continueDir, "sessions"),
		filepath.Join(continueDir, "logs"))

	fmt.Println("--------------------------------------------------------")
	logInfo("GLOBAL CONFIGURATION COMPLETED SUCCESSFULLY")
	fmt.Println("--------------------------------------------------------")
	logInfo("1. Restart VS Code COMPLETELY")
	logInfo("2. Open the Continue panel (right sidebar)")
	logInfo("3. Select the model: ollama-chat")
	fmt.Println("--------------------------------------------------------")
}

// --- fim do arquivo polyglot (ollama.go/ollama.sh) ---

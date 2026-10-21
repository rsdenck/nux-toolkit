/* 2>/dev/null
# ---------------------------------------------------------------------------
#  analyzer.sh — polyglot: este mesmo arquivo é analyzer.go (Go) e analyzer.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run analyzer.go [argumentos]
#    Bash ...: bash analyzer.sh  [argumentos]   (exige a toolchain Go)
#
#  Cabeçalho original do script:
#  =========================================================
#  CEPH CLUSTER ANALYZER - SRE TOOL
#  =========================================================
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[analyzer.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[analyzer.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-analyzer"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/analyzer.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init analyzer >/dev/null 2>&1) || :

_bin="$_trab/analyzer"
if [ ! -x "$_bin" ] || [ "$_trab/analyzer.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando analyzer.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o analyzer.tmp analyzer.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o analyzer.tmp analyzer.go) ||
			_die "falha ao compilar $_trab/analyzer.go"
	}
	mv -f "$_trab/analyzer.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (analyzer.go/analyzer.sh) ---'
*/
// analyzer.go — analisador interativo de cluster Ceph (SRE) com menu whiptail.
//
// Porta fiel do script original analyzer.sh: menu whiptail em loop com score
// de saúde (100 − OSDs down×10 − PGs degraded×5 − undersized×5, piso 0),
// comandos "ceph" rodando dentro do pod deploy/rook-ceph-tools via
// `kubectl -n rook-ceph exec …`, pausa "Pressione ENTER para continuar..." e
// a criação de /tmp/ceph_analyzer (fiel, mesmo sem uso). Mantém o
// set -euo pipefail: falha não tratada encerra com o mesmo código de saída
// do comando que falhou. Nenhum texto de saída foi traduzido ou alterado.
//
// Execução:
//
//	go run analyzer.go
//	bash analyzer.sh   (mesmo arquivo, polyglot Go + sh)
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Constantes e globals
// ---------------------------------------------------------------------------

const (
	cephNS   = "rook-ceph"
	toolsPod = "deploy/rook-ceph-tools"
	tmpAnl   = "/tmp/ceph_analyzer"
)

// entrada é compartilhada entre as pausas para não descartar o que o bufio
// já tiver lido do stdin.
var entrada = bufio.NewReader(os.Stdin)

// ---------------------------------------------------------------------------
// UTILS (run_ceph / pause / clear) e helpers de pipeline
// ---------------------------------------------------------------------------

// morrer reproduz o set -e: a saída de erro do comando já foi repassada ao
// terminal; sai com o mesmo código de saída do comando que falhou.
func morrer(err error) {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		os.Exit(ee.ExitCode())
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// runCEPH é o run_ceph original:
// kubectl -n "$CEPH_NS" exec "$TOOLS_POD" -- "$@"
func runCEPH(args ...string) {
	cmd := exec.Command("sh", append([]string{"-c",
		`kubectl -n rook-ceph exec deploy/rook-ceph-tools -- "$@"`, "sh"}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
}

// comandoPipe embrulha o pipeline verbatim com o pipefail do set -o pipefail
// (bash, como o original; sh como fallback).
func comandoPipe(script string) *exec.Cmd {
	if _, err := exec.LookPath("bash"); err == nil {
		return exec.Command("bash", "-c", "set -o pipefail; "+script)
	}
	return exec.Command("sh", "-c", script)
}

// shPipeT roda um pipeline verbatim do original com saída em tempo real;
// falha não tratada encerra o programa (set -e + pipefail).
func shPipeT(script string) {
	cmd := comandoPipe(script)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
}

// shPipe roda um pipeline verbatim capturando o stdout como $(...) faria;
// falha não tratada encerra o programa (set -e + pipefail).
func shPipe(script string) string {
	cmd := comandoPipe(script)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
	return strings.TrimRight(buf.String(), "\n")
}

// pause é o `read -rp "Pressione ENTER para continuar..."` do original: o
// prompt vai para o stderr e só aparece com stdin em terminal (como o bash);
// EOF no stdin encerra com código 1 (o set -e abortaria no read).
func pause() {
	if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		fmt.Fprint(os.Stderr, "Pressione ENTER para continuar...")
	}
	if _, err := entrada.ReadString('\n'); err != nil {
		os.Exit(1)
	}
}

// limpar é o `clear` original (falha = aborto, como no set -e).
func limpar() {
	cmd := exec.Command("clear")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
}

// ---------------------------------------------------------------------------
// HEALTH SCORE
// ---------------------------------------------------------------------------

// intArith converte a saída numérica do awk para o (( ... )) do original.
// Valor não numérico é inatingível na prática: com pipefail o grep sem
// match já abortou antes, então a queda para 0 nunca acontece.
func intArith(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func healthScore() int {
	score := 100

	osdDown := shPipe(`kubectl -n rook-ceph exec deploy/rook-ceph-tools -- ceph osd stat | grep -o '[0-9]\+ osds down' | awk '{print $1}'`)

	pgDegraded := shPipe(`kubectl -n rook-ceph exec deploy/rook-ceph-tools -- ceph pg stat | grep -o '[0-9]\+ degraded' | awk '{print $1}'`)

	pgUndersized := shPipe(`kubectl -n rook-ceph exec deploy/rook-ceph-tools -- ceph pg stat | grep -o '[0-9]\+ undersized' | awk '{print $1}'`)

	score -= intArith(osdDown) * 10
	score -= intArith(pgDegraded) * 5
	score -= intArith(pgUndersized) * 5

	if score < 0 {
		score = 0
	}

	return score
}

// ---------------------------------------------------------------------------
// OSD ANALYSIS
// ---------------------------------------------------------------------------

func osdDownAnalysis() {
	limpar()
	fmt.Println("OSDs DOWN - ANÁLISE DETALHADA")
	fmt.Println("--------------------------------")

	shPipeT(`kubectl -n rook-ceph exec deploy/rook-ceph-tools -- ceph osd tree | awk '$4=="down" {print "OSD:",$1,"HOST:",$7}'`)

	fmt.Println()
	fmt.Println("Detalhes técnicos:")
	shPipeT(`kubectl -n rook-ceph exec deploy/rook-ceph-tools -- ceph health detail | grep -i osd || true`)

	pause()
}

// ---------------------------------------------------------------------------
// PG ANALYSIS
// ---------------------------------------------------------------------------

func pgAnalysis() {
	limpar()
	fmt.Println("PLACEMENT GROUPS - ESTADO")
	fmt.Println("--------------------------------")

	runCEPH("ceph", "pg", "stat")

	fmt.Println()
	fmt.Println("PGs problemáticos:")
	shPipeT(`kubectl -n rook-ceph exec deploy/rook-ceph-tools -- ceph pg dump | awk '$16!="active+clean" {print $1,$16}' | head -20`)

	pause()
}

// ---------------------------------------------------------------------------
// PG ↔ OSD CORRELATION
// ---------------------------------------------------------------------------

func pgOsdMap() {
	limpar()
	fmt.Println("CORRELAÇÃO PG → OSD")
	fmt.Println("--------------------------------")

	shPipeT(`kubectl -n rook-ceph exec deploy/rook-ceph-tools -- ceph pg dump | awk '
    NR>1 {
      split($15, osds, ",")
      print "PG:",$1,"| STATE:",$16,"| OSDs:",$15
    }' | head -20`)

	pause()
}

// ---------------------------------------------------------------------------
// RBD LATENCY
// ---------------------------------------------------------------------------

func rbdLatency() {
	limpar()
	fmt.Println("LATÊNCIA RBD (OSD PERF)")
	fmt.Println("--------------------------------")

	runCEPH("ceph", "osd", "perf")

	pause()
}

// ---------------------------------------------------------------------------
// POOL ANALYSIS
// ---------------------------------------------------------------------------

func poolAnalysis() {
	limpar()
	fmt.Println("POOLS E PGs")
	fmt.Println("--------------------------------")

	shPipeT(`kubectl -n rook-ceph exec deploy/rook-ceph-tools -- ceph osd pool ls detail | egrep 'pool|pg_num|size|min_size'`)

	pause()
}

// ---------------------------------------------------------------------------
// SUMMARY
// ---------------------------------------------------------------------------

func summary() {
	limpar()
	score := healthScore()

	fmt.Println("RESUMO DO CLUSTER")
	fmt.Println("--------------------------------")
	fmt.Println("Health Score: " + strconv.Itoa(score) + " / 100")
	fmt.Println()

	runCEPH("ceph", "status")

	fmt.Println()
	fmt.Println("Interpretação:")
	switch {
	case score >= 90:
		fmt.Println("Cluster saudável")
	case score >= 70:
		fmt.Println("Cluster degradado - atenção")
	case score >= 40:
		fmt.Println("Cluster crítico - risco operacional")
	default:
		fmt.Println("Cluster instável - ação imediata necessária")
	}

	pause()
}

// ---------------------------------------------------------------------------
// MENU
// ---------------------------------------------------------------------------

// menu roda o whiptail com os MESMOS argumentos do original e reproduz o
// redirecionamento `3>&1 1>&2 2>&3` dentro do próprio sh: a seleção (stderr
// do whiptail) cai no pipe de captura (CHOICE) e o desenho (stdout) vai para
// o stderr do script. Falha do whiptail (Esc/Cancel) aborta como no set -e.
func menu() string {
	cmd := exec.Command("sh", "-c", `whiptail --title "Ceph Cluster Analyzer" \
    --menu "Selecione a análise desejada" 20 70 10 \
    "1" "Resumo e Health Score do Cluster" \
    "2" "OSDs DOWN - Causa e impacto" \
    "3" "PGs Degraded / Undersized" \
    "4" "Correlação PG ↔ OSD" \
    "5" "Latência RBD (OSD Perf)" \
    "6" "Pools e Dimensionamento de PGs" \
    "0" "Sair" 3>&1 1>&2 2>&3`)
	var captura bytes.Buffer
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stderr
	cmd.Stderr = &captura
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
	return strings.TrimRight(captura.String(), "\n")
}

func main() {
	// mkdir -p "$TMP" — fiel, mesmo sem uso depois (como no original)
	mk := exec.Command("sh", "-c", "mkdir -p "+tmpAnl)
	mk.Stdout, mk.Stderr = os.Stdout, os.Stderr
	if err := mk.Run(); err != nil {
		morrer(err)
	}

	for {
		choice := menu()
		switch choice {
		case "1":
			summary()
		case "2":
			osdDownAnalysis()
		case "3":
			pgAnalysis()
		case "4":
			pgOsdMap()
		case "5":
			rbdLatency()
		case "6":
			poolAnalysis()
		case "0":
			os.Exit(0)
		}
	}
}

// --- fim do arquivo polyglot (analyzer.go/analyzer.sh) ---

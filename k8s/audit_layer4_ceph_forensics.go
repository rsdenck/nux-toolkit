/* 2>/dev/null
# ---------------------------------------------------------------------------
#  audit_layer4_ceph_forensics.sh — polyglot: este mesmo arquivo é
#  audit_layer4_ceph_forensics.go (Go) e audit_layer4_ceph_forensics.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run audit_layer4_ceph_forensics.go [argumentos]
#    Bash ...: bash audit_layer4_ceph_forensics.sh  [argumentos]   (exige a toolchain Go)
#
#  Cabeçalho original do script:
#  ==============================================================================
#  AUDITORIA DE INFRAESTRUTURA CRÍTICA - CEPH/K8S
#  CAMADA 4: CEPH (MODO FORENSE)
#  OBJETIVO: Diagnosticar PGs e OSDs sem disparar recovery ou scrub.
#  ==============================================================================
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[audit_layer4_ceph_forensics.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[audit_layer4_ceph_forensics.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-audit_layer4_ceph_forensics"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/audit_layer4_ceph_forensics.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init audit_layer4_ceph_forensics >/dev/null 2>&1) || :

_bin="$_trab/audit_layer4_ceph_forensics"
if [ ! -x "$_bin" ] || [ "$_trab/audit_layer4_ceph_forensics.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando audit_layer4_ceph_forensics.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o audit_layer4_ceph_forensics.tmp audit_layer4_ceph_forensics.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o audit_layer4_ceph_forensics.tmp audit_layer4_ceph_forensics.go) ||
			_die "falha ao compilar $_trab/audit_layer4_ceph_forensics.go"
	}
	mv -f "$_trab/audit_layer4_ceph_forensics.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (audit_layer4_ceph_forensics.go/audit_layer4_ceph_forensics.sh) ---'
*/
// audit_layer4_ceph_forensics.go — auditoria da camada 4 (Ceph, modo forense).
//
// Porta fiel do script original audit_layer4_ceph_forensics.sh: diagnostica
// PGs e OSDs sem disparar recovery ou scrub — health/status, osd tree/dump,
// resumo de PGs sem dump completo, investigação de PGs unfound/down, osd df
// e flags de scrub/recovery. Exige o CLI 'ceph' no PATH (senão loga o ERRO
// do original e sai com 1). Cada linha passa por
// `tee -a audit_layer4_<timestamp>.log` (arquivo no diretório corrente) e
// pelo console. Mantém o set -u SEM set -e: falhas de pipeline não abortam,
// como no original. Nenhum texto de saída foi traduzido ou alterado.
//
// Execução:
//
//	go run audit_layer4_ceph_forensics.go
//	bash audit_layer4_ceph_forensics.sh   (mesmo arquivo, polyglot Go + sh)
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Infraestrutura fiel ao original (set -u, sem set -e)
// ---------------------------------------------------------------------------

var logFile string

// shc roda um trecho verbatim do original via sh -c, com saída em tempo real.
// Sem set -e, a falha não aborta: o status é ignorado, como no original.
func shc(script string, args ...string) {
	cmd := exec.Command("sh", append([]string{"-c", script, "sh"}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = cmd.Run()
}

// shcErr roda um trecho verbatim e devolve o status — é o que decide o
// "|| log ..." do original.
func shcErr(script string, args ...string) error {
	cmd := exec.Command("sh", append([]string{"-c", script, "sh"}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// shcOut roda um trecho verbatim e devolve o stdout como $(...) devolveria
// (sem os quebras de linha finais). Falha não aborta (sem set -e).
func shcOut(script string, args ...string) string {
	cmd := exec.Command("sh", append([]string{"-c", script, "sh"}, args...)...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	return strings.TrimRight(buf.String(), "\n")
}

// log reproduz exatamente o original:
// echo -e "[$(date +'%Y-%m-%d %H:%M:%S')] $1" | tee -a "$LOG_FILE"
func log(msg string) {
	linha := fmt.Sprintf("[%s] %s", time.Now().Format("2006-01-02 15:04:05"), msg)
	shc(`echo -e "$1" | tee -a "$2"`, linha, logFile)
}

func header(txt string) {
	log("======================================================================")
	log(txt)
	log("======================================================================")
}

func main() {
	timestamp := time.Now().Format("20060102_150405")
	logFile = "audit_layer4_" + timestamp + ".log"

	// Verificar se o comando ceph está disponível
	if _, err := exec.LookPath("ceph"); err != nil {
		log("ERRO: Comando 'ceph' não encontrado. Este script deve rodar onde o CLI do Ceph esteja configurado.")
		os.Exit(1)
	}

	header("INICIANDO AUDITORIA CAMADA 4 - CEPH (MODO FORENSE)")

	// 1. Status Geral (Execução Única)
	header("1. CEPH HEALTH & STATUS")
	log("Coletando status resumido:")
	shc(`ceph status --format plain | tee -a "$1"`, logFile)

	log("Detalhes de saúde (Health Detail):")
	shc(`ceph health detail | tee -a "$1"`, logFile)

	// 2. Hierarquia e Estado de OSDs
	header("2. OSD TREE & FLAPPING DETECTION")
	log("Verificando topologia e OSDs DOWN/IN:")
	shc(`ceph osd tree | tee -a "$1"`, logFile)

	log("Buscando OSDs com alto índice de reweights ou flapping (recentes):")
	shc(`ceph osd dump | grep -i "flags" | tee -a "$1"`, logFile)

	// 3. Análise de Placement Groups (PGs) - MODO SEGURO
	header("3. PG SUMMARY (NON-DESTRUCTIVE)")
	log("Resumo de estados de PGs (Sem dump completo):")
	shc(`ceph pg stat | tee -a "$1"`, logFile)

	log("Identificando PGs em estados críticos (stale, undersized, degraded, unfound):")
	shc(`ceph pg ls-by-primary 2>/dev/null | grep -Ei "stale|undersized|degraded|unfound|inactive|down" | head -n 20 | tee -a "$1"`, logFile)

	// 4. Localização de Dados Críticos (PG 44.2 e similares)
	header("4. INVESTIGAÇÃO DE PGs PROBLEMÁTICAS")
	criticalPgs := shcOut(`ceph pg ls | grep -Ei "unfound|down" | awk '{print $1}' | head -n 5`)
	if criticalPgs != "" {
		for _, pg := range strings.Fields(criticalPgs) {
			log("Analisando localização da PG " + pg + ":")
			shc(`ceph pg "$1" query | grep -E "acting|up|blocked_by" | tee -a "$2"`, pg, logFile)
		}
	} else {
		log("Nenhuma PG em estado 'unfound' ou 'down' detectada nos primeiros registros.")
	}

	// 5. Uso de Espaço e Performance de IO
	header("5. OSD DF & UTILIZATION")
	log("Verificando preenchimento dos OSDs (Cuidado com Nearfull):")
	shc(`ceph osd df | sort -nk 8 | tail -n 10 | tee -a "$1"`, logFile)

	// 6. Verificação de Configurações de Risco (Scrub/Recovery)
	header("6. RECOVERY & SCRUB CONFIGURATION")
	log("Verificando se flags de pausa estão ativas (recomendado em crises):")
	// O "|| log ..." só dispara se o pipeline todo falhar — sem pipefail o
	// status é o do tee; mantido fielmente (guarda efetivamente morta).
	if err := shcErr(`ceph osd dump | grep -E "noscrub|nodeep-scrub|norecover|nobackfill" | tee -a "$1"`, logFile); err != nil {
		log("AVISO: Flags de proteção (noscrub/norecover) NÃO estão ativas.")
	}

	log("======================================================================")
	log("AUDITORIA DA CAMADA 4 FINALIZADA.")
	log("Logs salvos em: " + logFile)
	log("======================================================================")
}

// --- fim do arquivo polyglot (audit_layer4_ceph_forensics.go/audit_layer4_ceph_forensics.sh) ---

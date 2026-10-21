/* 2>/dev/null
# ---------------------------------------------------------------------------
#  audit_layer5_bluestore_rocksdb.sh — polyglot: este mesmo arquivo é
#  audit_layer5_bluestore_rocksdb.go (Go) e audit_layer5_bluestore_rocksdb.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run audit_layer5_bluestore_rocksdb.go [argumentos]
#    Bash ...: bash audit_layer5_bluestore_rocksdb.sh  [argumentos]   (exige a toolchain Go)
#
#  Cabeçalho original do script:
#  ==============================================================================
#  AUDITORIA DE INFRAESTRUTURA CRÍTICA - CEPH/K8S
#  CAMADA 5: ROCKSDB / BLUESTORE (INDIRETO)
#  OBJETIVO: Detectar corrupção de metadados, saturação de WAL/DB e cruzamento de IDs.
#  ==============================================================================
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[audit_layer5_bluestore_rocksdb.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[audit_layer5_bluestore_rocksdb.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-audit_layer5_bluestore_rocksdb"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/audit_layer5_bluestore_rocksdb.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init audit_layer5_bluestore_rocksdb >/dev/null 2>&1) || :

_bin="$_trab/audit_layer5_bluestore_rocksdb"
if [ ! -x "$_bin" ] || [ "$_trab/audit_layer5_bluestore_rocksdb.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando audit_layer5_bluestore_rocksdb.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o audit_layer5_bluestore_rocksdb.tmp audit_layer5_bluestore_rocksdb.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o audit_layer5_bluestore_rocksdb.tmp audit_layer5_bluestore_rocksdb.go) ||
			_die "falha ao compilar $_trab/audit_layer5_bluestore_rocksdb.go"
	}
	mv -f "$_trab/audit_layer5_bluestore_rocksdb.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (audit_layer5_bluestore_rocksdb.go/audit_layer5_bluestore_rocksdb.sh) ---'
*/
// audit_layer5_bluestore_rocksdb.go — auditoria da camada 5 (RocksDB/BlueStore).
//
// Porta fiel do script original audit_layer5_bluestore_rocksdb.sh: cruza
// OSD -> disco -> LVM via ceph-volume (ou lsblk), procura assinaturas de
// corrupção/saturação de WAL nos logs de /var/log/ceph, mostra a latência
// interna do OSD (ceph osd perf), investiga os OSDs 2 e 4 e estima o uso do
// block.db com df. Mantém o set -u SEM set -e: falhas de pipeline não
// abortam, como no original, e cada linha passa por
// `tee -a audit_layer5_<timestamp>.log` (arquivo no diretório corrente) e
// pelo console. Nenhum texto de saída foi traduzido ou alterado.
//
// Execução:
//
//	go run audit_layer5_bluestore_rocksdb.go
//	bash audit_layer5_bluestore_rocksdb.sh   (mesmo arquivo, polyglot Go + sh)
package main

import (
	"fmt"
	"os"
	"os/exec"
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

// dirExiste reproduz os testes "[ -d "$LOG_DIR" ]" do original.
func dirExiste(caminho string) bool {
	st, err := os.Stat(caminho)
	return err == nil && st.IsDir()
}

func main() {
	timestamp := time.Now().Format("20060102_150405")
	logFile = "audit_layer5_" + timestamp + ".log"

	header("INICIANDO AUDITORIA CAMADA 5 - ROCKSDB / BLUESTORE")

	// 1. Cruzamento de IDs: OSD -> Device -> LVM
	header("1. MAPEAMENTO DE INFRAESTRUTURA (OSD -> DISCO)")
	if _, err := exec.LookPath("ceph-volume"); err == nil {
		log("Mapeamento lógico BlueStore (via ceph-volume):")
		shc(`ceph-volume lvm list | grep -E "osd id|devices|block.db device|block.wal device" | tee -a "$1"`, logFile)
	} else {
		log("ceph-volume não encontrado. Tentando mapeamento via lsblk:")
		shc(`lsblk -o NAME,SIZE,MOUNTPOINT,PARTLABEL | grep -i "ceph" | tee -a "$1"`, logFile)
	}

	// 2. Análise de Logs de Erros de BlueStore/RocksDB (SEM LEITURA PROFUNDA)
	header("2. DETECÇÃO DE CORRUPÇÃO E WAL SATURATION")
	logDir := "/var/log/ceph"
	if dirExiste(logDir) {
		log("Buscando assinaturas de falha nos logs existentes (últimas 1000 linhas):")

		// Checksum errors - Sinal claro de falha de disco ou bit-rot
		log(" - Verificando Checksum Errors (BlueStore):")
		shc(`grep -rEi "checksum|crc|corrupt" "$1" --include="ceph-osd.*.log" | tail -n 10 | tee -a "$2"`, logDir, logFile)

		// RocksDB Compaction/WAL issues
		log(" - Verificando Saturação de WAL/RocksDB:")
		shc(`grep -rEi "RocksDB|compaction|slow request|waiting for|failed to write to" "$1" --include="ceph-osd.*.log" | tail -n 10 | tee -a "$2"`, logDir, logFile)

		// BlueStore Allocation failures
		log(" - Verificando Erros de Alocação (BlueStore):")
		shc(`grep -rEi "enospc|allocation failed|failed to allocate" "$1" --include="ceph-osd.*.log" | tail -n 10 | tee -a "$2"`, logDir, logFile)
	} else {
		log("Diretório de logs " + logDir + " não encontrado ou inacessível.")
	}

	// 3. Métricas de Performance Internas (Via Ceph OSD Perf)
	header("3. MÉTRICAS DE LATÊNCIA INTERNA (OSD PERF)")
	if _, err := exec.LookPath("ceph"); err == nil {
		log("Latência de Commit e Apply (ms):")
		// Analisando latência de commit (WAL) e apply (DB/Block)
		shc(`ceph osd perf | sort -nk 2 | tail -n 15 | tee -a "$1"`, logFile)
	} else {
		log("Comando ceph não disponível para métricas de performance.")
	}

	// 4. Cruzamento de IDs críticos (OSD 2 e 4)
	header("4. FORENSE DE OSDs ALVO (2 e 4)")
	for _, id := range []string{"2", "4"} {
		log("Análise específica para OSD." + id + ":")
		if dirExiste(logDir) {
			log(" - Últimos eventos críticos no log do OSD." + id + ":")
			shc(`find "$1" -name "ceph-osd.$2.log*" -exec tail -n 20 {} + | grep -Ei "error|fail|reset|compact|slow" | tee -a "$3"`, logDir, id, logFile)
		}

		// Verifica se há PGs unfound ligadas a estes IDs (Baseado na Camada 4)
		log(" - Verificando se OSD." + id + " é o primário de PGs em 'unfound':")
		if _, err := exec.LookPath("ceph"); err == nil {
			// Sem pipefail o status é o do tee; mantido fielmente
			// (guarda efetivamente morta, como no original).
			if err := shcErr(`ceph pg ls | grep "^$1\." | grep "unfound" | tee -a "$2"`, id, logFile); err != nil {
				log("Nenhuma PG unfound como primário no OSD." + id + ".")
			}
		}
	}

	// 5. Verificação de Fragmentation (Indireto)
	header("5. FRAGMENTAÇÃO E DB SIZE (ESTIMATIVA)")
	log("Verificando se block.db está próximo do limite (se separado):")
	shc(`df -h | grep -i "ceph" | tee -a "$1"`, logFile)

	log("======================================================================")
	log("AUDITORIA DA CAMADA 5 FINALIZADA.")
	log("Logs salvos em: " + logFile)
	log("======================================================================")
}

// --- fim do arquivo polyglot (audit_layer5_bluestore_rocksdb.go/audit_layer5_bluestore_rocksdb.sh) ---

/* 2>/dev/null
# ---------------------------------------------------------------------------
#  audit_layer2_os_health.sh — polyglot: este mesmo arquivo é
#  audit_layer2_os_health.go (Go) e audit_layer2_os_health.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run audit_layer2_os_health.go [argumentos]
#    Bash ...: bash audit_layer2_os_health.sh  [argumentos]   (exige a toolchain Go)
#
#  Cabeçalho original do script:
#  ==============================================================================
#  AUDITORIA DE INFRAESTRUTURA CRÍTICA - CEPH/K8S
#  CAMADA 2: SISTEMA OPERACIONAL (RECURSOS E KERNEL)
#  OBJETIVO: Avaliar saturação de CPU, Memória e estabilidade do Kernel.
#  ==============================================================================
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[audit_layer2_os_health.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[audit_layer2_os_health.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-audit_layer2_os_health"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/audit_layer2_os_health.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init audit_layer2_os_health >/dev/null 2>&1) || :

_bin="$_trab/audit_layer2_os_health"
if [ ! -x "$_bin" ] || [ "$_trab/audit_layer2_os_health.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando audit_layer2_os_health.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o audit_layer2_os_health.tmp audit_layer2_os_health.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o audit_layer2_os_health.tmp audit_layer2_os_health.go) ||
			_die "falha ao compilar $_trab/audit_layer2_os_health.go"
	}
	mv -f "$_trab/audit_layer2_os_health.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (audit_layer2_os_health.go/audit_layer2_os_health.sh) ---'
*/
// audit_layer2_os_health.go — auditoria da camada 2 (SO: recursos e kernel).
//
// Porta fiel do script original audit_layer2_os_health.sh: avalia saturação
// de CPU, memória e estabilidade do kernel — load average, context switches,
// PSI (pressão de CPU/memória), OOM/panics no dmesg, limites de cgroups,
// NUMA/HugePages e softnet. Cada linha passa por
// `tee -a audit_layer2_<timestamp>.log` (arquivo no diretório corrente) e
// pelo console. Mantém o set -u SEM set -e: falhas de pipeline não abortam,
// como no original. Nenhum texto de saída foi traduzido ou alterado.
//
// Execução:
//
//	go run audit_layer2_os_health.go
//	bash audit_layer2_os_health.sh   (mesmo arquivo, polyglot Go + sh)
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

// arquivoRegular é o teste [ -f ... ] do original.
func arquivoRegular(caminho string) bool {
	fi, err := os.Stat(caminho)
	return err == nil && fi.Mode().IsRegular()
}

// dirExiste é o teste [ -d ... ] do original.
func dirExiste(caminho string) bool {
	fi, err := os.Stat(caminho)
	return err == nil && fi.IsDir()
}

func main() {
	timestamp := time.Now().Format("20060102_150405")
	logFile = "audit_layer2_" + timestamp + ".log"

	header("INICIANDO AUDITORIA CAMADA 2 - SISTEMA OPERACIONAL")

	// 1. Load Average e Context Switches
	header("1. CPU LOAD E CONTEXT SWITCHES")
	log("Load Average (1m, 5m, 15m):")
	shc(`cat /proc/loadavg | tee -a "$1"`, logFile)

	log("Context Switches e Migrações (via /proc/stat):")
	shc(`grep -E "ctxt|processes|procs_running" /proc/stat | tee -a "$1"`, logFile)

	// 2. CPU Pressure (PSI) - Latência de agendamento
	header("2. CPU PRESSURE (PSI)")
	if arquivoRegular("/proc/pressure/cpu") {
		log("Verificando contenção de CPU:")
		shc(`cat /proc/pressure/cpu | tee -a "$1"`, logFile)
	} else {
		log("PSI CPU não disponível.")
	}

	// 3. Memória e Saturação (PSI)
	header("3. MEMÓRIA E SWAP")
	log("Resumo de Memória:")
	shc(`grep -E "MemTotal|MemFree|MemAvailable|Buffers|Cached|SwapTotal|SwapFree|SReclaimable" /proc/meminfo | tee -a "$1"`, logFile)

	if arquivoRegular("/proc/pressure/memory") {
		log("Verificando contenção de Memória (PSI):")
		shc(`cat /proc/pressure/memory | tee -a "$1"`, logFile)
	} else {
		log("PSI Memory não disponível.")
	}

	// 4. Kernel - OOM Killer e Panics
	header("4. ESTABILIDADE DO KERNEL (LOGS)")
	log("Buscando por OOM Killer, Segfaults ou Kernel Panics recentes:")
	shc(`dmesg -T | grep -Ei "oom-killer|out of memory|segmentation fault|panic|tainted|soft lockup" | tail -n 20 | tee -a "$1"`, logFile)

	// 5. Cgroups e Resource Limits (Leitura)
	header("5. LIMITES DE RECURSOS (CGROUPS)")
	log("Verificando Memory High/Max em cgroups (possíveis gargalos K8s):")
	shc(`find /sys/fs/cgroup -name "memory.high" -exec grep -Hv "max" {} + 2>/dev/null | head -n 10 | tee -a "$1"`, logFile)
	shc(`find /sys/fs/cgroup -name "memory.max" -exec grep -Hv "max" {} + 2>/dev/null | head -n 10 | tee -a "$1"`, logFile)

	// 6. NUMA e HugePages (Check de performance passivo)
	header("6. NUMA E HUGEPAGES")
	if dirExiste("/sys/devices/system/node") {
		log("Nodes NUMA detectados: " + shcOut(`ls -d /sys/devices/system/node/node* | wc -l`))
		shc(`grep "" /sys/kernel/mm/transparent_hugepage/enabled | tee -a "$1"`, logFile)
	}

	// 7. Network Stack (Check de latência/erros silenciosos)
	header("7. NETWORK STACK (SOFTNET STATS)")
	log("Verificando quedas de pacotes no processamento de interrupções (Softnet):")
	// Coluna 1: processados, Coluna 2: dropados, Coluna 3: time_squeeze
	shc(`awk '{print "CPU"NR-1": Processed: "$1", Dropped: "$2", TimeSqueeze: "$3}' /proc/net/softnet_stat | tee -a "$1"`, logFile)

	log("======================================================================")
	log("AUDITORIA DA CAMADA 2 FINALIZADA.")
	log("Logs salvos em: " + logFile)
	log("======================================================================")
}

// --- fim do arquivo polyglot (audit_layer2_os_health.go/audit_layer2_os_health.sh) ---

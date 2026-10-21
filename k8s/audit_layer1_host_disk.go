/* 2>/dev/null
# ---------------------------------------------------------------------------
#  audit_layer1_host_disk.sh — polyglot: este mesmo arquivo é
#  audit_layer1_host_disk.go (Go) e audit_layer1_host_disk.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run audit_layer1_host_disk.go [argumentos]
#    Bash ...: bash audit_layer1_host_disk.sh  [argumentos]   (exige a toolchain Go)
#
#  Cabeçalho original do script:
#  ==============================================================================
#  AUDITORIA DE INFRAESTRUTURA CRÍTICA - CEPH/K8S
#  CAMADA 1: VM / HOST (DISCO E SO)
#  OBJETIVO: Coleta de métricas de saúde de disco com IO Mínimo.
#  ==============================================================================
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[audit_layer1_host_disk.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[audit_layer1_host_disk.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-audit_layer1_host_disk"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/audit_layer1_host_disk.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init audit_layer1_host_disk >/dev/null 2>&1) || :

_bin="$_trab/audit_layer1_host_disk"
if [ ! -x "$_bin" ] || [ "$_trab/audit_layer1_host_disk.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando audit_layer1_host_disk.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o audit_layer1_host_disk.tmp audit_layer1_host_disk.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o audit_layer1_host_disk.tmp audit_layer1_host_disk.go) ||
			_die "falha ao compilar $_trab/audit_layer1_host_disk.go"
	}
	mv -f "$_trab/audit_layer1_host_disk.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (audit_layer1_host_disk.go/audit_layer1_host_disk.sh) ---'
*/
// audit_layer1_host_disk.go — auditoria da camada 1 (VM/host: disco e SO).
//
// Porta fiel do script original audit_layer1_host_disk.sh: coleta métricas de
// saúde de disco com I/O mínimo — /proc/pressure/io, montagens read-only,
// estatísticas de /sys/block, dmesg, smartctl passivo e limites de I/O em
// cgroups. Cada linha passa por `tee -a audit_layer1_<timestamp>.log`
// (arquivo no diretório corrente) e pelo console. Mantém o set -u SEM set -e:
// falhas de pipeline não abortam, como no original. Nenhum texto de saída
// foi traduzido ou alterado.
//
// Execução:
//
//	go run audit_layer1_host_disk.go
//	bash audit_layer1_host_disk.sh   (mesmo arquivo, polyglot Go + sh)
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

// arquivoRegular é o teste [ -f ... ] do original.
func arquivoRegular(caminho string) bool {
	fi, err := os.Stat(caminho)
	return err == nil && fi.Mode().IsRegular()
}

func main() {
	timestamp := time.Now().Format("20060102_150405")
	logFile = "audit_layer1_" + timestamp + ".log"

	header("INICIANDO AUDITORIA CAMADA 1 - VM/HOST (DISCO E SO)")

	// 1. Verificação de PSI (Pressure Stall Information) - Latência de IO em nível de Kernel
	header("1. IO PRESSURE (PSI)")
	if arquivoRegular("/proc/pressure/io") {
		log("Analisando gargalo de IO via /proc/pressure/io:")
		shc(`cat /proc/pressure/io | tee -a "$1"`, logFile)
	} else {
		log("PSI não disponível neste Kernel.")
	}

	// 2. Verificação de Discos em Read-Only (Sinal de falha grave de FS)
	header("2. FILESYSTEMS READ-ONLY")
	log("Procurando por montagens em modo RO (Read-Only):")
	// O "|| log ..." só dispara se o pipeline todo falhar — sem pipefail o
	// status é o do tee; mantido fielmente (guarda efetivamente morta).
	if err := shcErr(`grep " ro," /proc/mounts | grep -v "/sys" | grep -v "/proc" | tee -a "$1"`, logFile); err != nil {
		log("Nenhum FS em RO detectado.")
	}

	// 3. Estatísticas de Bloco (/sys/block) - Sem leitura de disco
	header("3. ESTATÍSTICAS DE DISPOSITIVO DE BLOCO (MÉTRICAS KERNEL)")
	discos := shcOut(`for disk in /sys/block/[sh]d[a-z] /sys/block/nvme[0-9]n[1-9]; do [ -d "$disk" ] || continue; echo "$disk"; done`)
	for _, disk := range strings.Fields(discos) {
		diskName := shcOut(`basename "$1"`, disk)
		log("Analisando " + diskName + ":")

		// stat file format: read I/Os, read merges, read sectors, read ticks,
		// write I/Os, write merges, write sectors, write ticks,
		// in_flight, io_ticks, time_in_queue
		stats := shcOut(`cat "$1/stat"`, disk)
		readTicks := shcOut(`echo "$1" | awk '{print $4}'`, stats)
		writeTicks := shcOut(`echo "$1" | awk '{print $8}'`, stats)
		ioTicks := shcOut(`echo "$1" | awk '{print $10}'`, stats)

		log(" - Read Ticks (ms): " + readTicks)
		log(" - Write Ticks (ms): " + writeTicks)
		log(" - IO Ticks (ms): " + ioTicks)

		// Verificar se há descartes/erros reportados pelo kernel
		if arquivoRegular(disk + "/device/io_err_cnt") {
			errCnt := shcOut(`cat "$1"`, disk+"/device/io_err_cnt")
			log(" - IO Error Count (Hardware): " + errCnt)
		}
	}

	// 4. Verificação de dmesg por erros de I/O recentes
	header("4. ERROS DE I/O NO DMESG")
	log("Buscando padrões de falha (I/O error, timeout, reset, journal errors):")
	shc(`dmesg -T | grep -Ei "i/o error|timeout|reset|journal error|bad sector|corrupt" | tail -n 20 | tee -a "$1"`, logFile)

	// 5. SMART - Apenas leitura de atributos (SEM TESTE ATIVO)
	header("5. SMART ATTRIBUTES (PASSIVE READ)")
	if _, err := exec.LookPath("smartctl"); err != nil {
		log("smartctl não encontrado. Pulando.")
	} else {
		blocos := shcOut(`for disk in /dev/[sh]d[a-z] /dev/nvme[0-9]n[1-9]; do [ -b "$disk" ] || continue; echo "$disk"; done`)
		for _, disk := range strings.Fields(blocos) {
			log("SMART Status para " + disk + ":")
			shc(`smartctl -H "$1" | grep "test result" | tee -a "$2"`, disk, logFile)
			// Coleta apenas atributos críticos sem forçar spin-up ou teste
			shc(`smartctl -A "$1" | grep -Ei "Reallocated_Sector_Ct|Current_Pending_Sector|Offline_Uncorrectable|Runtime_Bad_Block" | tee -a "$2"`, disk, logFile)
		}
	}

	// 6. Verificação de Throttling (Cgroups V1/V2)
	header("6. IO THROTTLING (CGROUPS)")
	log("Verificando se há limites de IO configurados que podem causar latência:")
	shc(`find /sys/fs/cgroup -name "io.max" -exec grep -Hv "max" {} + 2>/dev/null | tee -a "$1"`, logFile)
	shc(`find /sys/fs/cgroup -name "blkio.throttle.*" -exec grep -Hv "0" {} + 2>/dev/null | tee -a "$1"`, logFile)

	log("======================================================================")
	log("AUDITORIA DA CAMADA 1 FINALIZADA.")
	log("Logs salvos em: " + logFile)
	log("======================================================================")
}

// --- fim do arquivo polyglot (audit_layer1_host_disk.go/audit_layer1_host_disk.sh) ---

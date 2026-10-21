/* 2>/dev/null
# ---------------------------------------------------------------------------
#  audit_layer3_k8s_containers.sh — polyglot: este mesmo arquivo é
#  audit_layer3_k8s_containers.go (Go) e audit_layer3_k8s_containers.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run audit_layer3_k8s_containers.go [argumentos]
#    Bash ...: bash audit_layer3_k8s_containers.sh  [argumentos]   (exige a toolchain Go)
#
#  Cabeçalho original do script:
#  ==============================================================================
#  AUDITORIA DE INFRAESTRUTURA CRÍTICA - CEPH/K8S
#  CAMADA 3: CONTAINERS / KUBERNETES (LOW IMPACT)
#  OBJETIVO: Avaliar estabilidade dos Pods e do Runtime sem gerar carga de API.
#  ==============================================================================
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[audit_layer3_k8s_containers.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[audit_layer3_k8s_containers.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-audit_layer3_k8s_containers"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/audit_layer3_k8s_containers.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init audit_layer3_k8s_containers >/dev/null 2>&1) || :

_bin="$_trab/audit_layer3_k8s_containers"
if [ ! -x "$_bin" ] || [ "$_trab/audit_layer3_k8s_containers.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando audit_layer3_k8s_containers.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o audit_layer3_k8s_containers.tmp audit_layer3_k8s_containers.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o audit_layer3_k8s_containers.tmp audit_layer3_k8s_containers.go) ||
			_die "falha ao compilar $_trab/audit_layer3_k8s_containers.go"
	}
	mv -f "$_trab/audit_layer3_k8s_containers.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (audit_layer3_k8s_containers.go/audit_layer3_k8s_containers.sh) ---'
*/
// audit_layer3_k8s_containers.go — auditoria da camada 3 (containers/K8s).
//
// Porta fiel do script original audit_layer3_k8s_containers.sh: avalia
// estabilidade dos pods e do runtime sem gerar carga de API — estado do
// kubelet/containerd/cri-o/docker via systemctl, pods rook-ceph com restarts
// ou fora de Running, eventos de Warning/Eviction, erros no journal do
// runtime e device plugins do kubelet. Cada linha passa por
// `tee -a audit_layer3_<timestamp>.log` (arquivo no diretório corrente) e
// pelo console. Mantém o set -u SEM set -e: falhas de pipeline não abortam,
// como no original. Nenhum texto de saída foi traduzido ou alterado.
//
// Execução:
//
//	go run audit_layer3_k8s_containers.go
//	bash audit_layer3_k8s_containers.sh   (mesmo arquivo, polyglot Go + sh)
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

// dirExiste é o teste [ -d ... ] do original.
func dirExiste(caminho string) bool {
	fi, err := os.Stat(caminho)
	return err == nil && fi.IsDir()
}

func main() {
	timestamp := time.Now().Format("20060102_150405")
	logFile = "audit_layer3_" + timestamp + ".log"

	header("INICIANDO AUDITORIA CAMADA 3 - CONTAINERS / KUBERNETES")

	// 1. Estado dos Serviços Críticos no Node
	header("1. ESTADO DO KUBELET E RUNTIME")
	for _, service := range []string{"kubelet", "containerd", "cri-o", "docker"} {
		cmd := exec.Command("systemctl", "is-active", "--quiet", service)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if cmd.Run() == nil {
			log("[OK] " + service + " está ATIVO.")
		} else {
			log("[WARN] " + service + " não está ativo ou não instalado.")
		}
	}

	// 2. Saúde dos Pods Ceph (Consultas Rápidas)
	header("2. STATUS DOS PODS ROOK-CEPH")
	if _, err := exec.LookPath("kubectl"); err != nil {
		log("kubectl não encontrado no PATH. Pulando consulta de API.")
	} else {
		log("Listando Pods com RestartCount > 0 (Namespace: rook-ceph):")
		shc(`kubectl get pods -n rook-ceph -o custom-columns="NAME:.metadata.name,STATUS:.status.phase,RESTARTS:.status.containerStatuses[0].restartCount" | awk '$3 > 0 {print $0}' | tee -a "$1"`, logFile)

		log("Pods não-Running:")
		shc(`kubectl get pods -n rook-ceph | grep -v "Running" | tee -a "$1"`, logFile)
	}

	// 3. Eventos Críticos Recentes
	header("3. EVENTOS DE INFRAESTRUTURA (K8S)")
	if _, err := exec.LookPath("kubectl"); err == nil {
		log("Buscando eventos de Warning nos últimos 30m:")
		shc(`kubectl get events -n rook-ceph --field-selector type=Warning | tail -n 15 | tee -a "$1"`, logFile)

		log("Buscando eventos de Eviction ou FailedScheduling:")
		shc(`kubectl get events -A | grep -Ei "evicted|failedscheduling|failedmount|failedattach" | tail -n 10 | tee -a "$1"`, logFile)
	}

	// 4. Verificação de Logs de Runtime (Sem leitura profunda)
	header("4. ERROS NO LOG DO RUNTIME")
	log("Últimas 20 linhas de erro do journal para o runtime:")
	shc(`journalctl -u containerd -u cri-o -u docker --no-pager -p err -n 20 | tee -a "$1"`, logFile)

	// 5. Resource Limits no K8s (Verificar se há Throttling de CPU via Kubelet)
	header("5. KUBELET CPU MANAGER / MEMORY LIMITS")
	if dirExiste("/var/lib/kubelet/device-plugins") {
		log("Kubelet Device Plugins detectados (possível uso de SRIOV/GPU/Direct Assignment).")
	}

	log("======================================================================")
	log("AUDITORIA DA CAMADA 3 FINALIZADA.")
	log("Logs salvos em: " + logFile)
	log("======================================================================")
}

// --- fim do arquivo polyglot (audit_layer3_k8s_containers.go/audit_layer3_k8s_containers.sh) ---

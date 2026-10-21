/* 2>/dev/null
# ---------------------------------------------------------------------------
#  ceph_osd.sh — polyglot: este mesmo arquivo é ceph_osd.go (Go) e ceph_osd.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run ceph_osd.go [argumentos]
#    Bash ...: bash ceph_osd.sh  [argumentos]   (exige a toolchain Go)
#
#  Cabeçalho original do script (banner de saída):
#  ========================================================================
#     CEPH OSD & PG RESOURCE CONFLICT ANALYZER
#  ========================================================================
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[ceph_osd.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[ceph_osd.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-ceph_osd"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/ceph_osd.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init ceph_osd >/dev/null 2>&1) || :

_bin="$_trab/ceph_osd"
if [ ! -x "$_bin" ] || [ "$_trab/ceph_osd.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando ceph_osd.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o ceph_osd.tmp ceph_osd.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o ceph_osd.tmp ceph_osd.go) ||
			_die "falha ao compilar $_trab/ceph_osd.go"
	}
	mv -f "$_trab/ceph_osd.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (ceph_osd.go/ceph_osd.sh) ---'
*/
// ceph_osd.go — analisador de conflito de recursos Ceph (OSD & PG).
//
// Porta fiel do script original ceph_osd.sh: tabela de mapeamento de PGs do
// pool 44, localização física/status dos OSDs por worker, consumo de
// CPU/memória dos pods OSD (kubectl top — falha sem metrics-server, como no
// original) e a dica final de expansão do OSD 2. Sem nenhum `set`: falhas de
// comando não abortam, como no original. Os valores do pool (44), do OSD (2)
// e o histórico de expansão (436 GiB) vêm hardcoded do script original.
// Nenhum texto de saída foi traduzido ou alterado.
//
// Execução:
//
//	go run ceph_osd.go
//	bash ceph_osd.sh   (mesmo arquivo, polyglot Go + sh)
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ---------------------------------------------------------------------------
// Infraestrutura fiel ao original (sem set -u, sem set -e)
// ---------------------------------------------------------------------------

// shc roda um trecho verbatim do original via sh -c, com saída em tempo real.
// Falha não aborta (o original não tem set -e).
func shc(script string, args ...string) {
	cmd := exec.Command("sh", append([]string{"-c", script, "sh"}, args...)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	_ = cmd.Run()
}

// shcOut roda um trecho verbatim e devolve o stdout como $(...) devolveria
// (sem as quebras de linha finais). Falha não aborta (sem set -e).
func shcOut(script string, args ...string) string {
	cmd := exec.Command("sh", append([]string{"-c", script, "sh"}, args...)...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	return strings.TrimRight(buf.String(), "\n")
}

func main() {
	// Banner inicial (linhas 3-5 do original, sem comentário equivalente)
	fmt.Println("========================================================================")
	fmt.Println("   CEPH OSD & PG RESOURCE CONFLICT ANALYZER")
	fmt.Println("========================================================================")

	// 1. Tabela de Mapeamento PG -> OSD
	fmt.Print("\n[1] MAPEAMENTO DE PGs PROBLEMÁTICOS (Pool 44)\n")
	fmt.Println("------------------------------------------------------------------------")
	fmt.Printf("%-10s | %-15s | %-20s\n", "PG_ID", "STATE", "ACTING_OSDS")
	shc(`ceph pg dump_stuck | grep "^44" | awk '{printf "%-10s | %-15s | %-20s\n", $1, $2, $15}'`)

	// 2. Tabela de Localização de OSDs por Worker
	fmt.Print("\n[2] LOCALIZAÇÃO FÍSICA E STATUS DOS OSDS\n")
	fmt.Println("------------------------------------------------------------------------")
	fmt.Printf("%-10s | %-12s | %-15s | %-10s\n", "OSD_ID", "STATUS", "NODE", "POD_PHASE")
	shc(`kubectl get pods -n rook-ceph -l app=rook-ceph-osd -o custom-columns=NAME:.metadata.name,STATUS:.status.phase,NODE:.spec.nodeName | grep -E "osd-[0-9]+" | sort -k3 | while read line; do
    pod_name=$(echo $line | awk '{print $1}')
    osd_id=$(echo $pod_name | grep -oE "[0-9]+" | head -1)
    status=$(echo $line | awk '{print $2}')
    node=$(echo $line | awk '{print $3}')
    printf "osd.%-6s | %-12s | %-15s | %-10s\n" "$osd_id" "$status" "$node" "$status"
done`)

	// 3. Competição de Recursos no RocksDB (Monitoramento de CPU/Mem por Node)
	fmt.Print("\n[3] COMPETIÇÃO DE RECURSOS NOS NODES (TOP OSD CONSUMERS)\n")
	fmt.Println("------------------------------------------------------------------------")
	fmt.Printf("%-20s | %-10s | %-10s | %-10s\n", "POD_NAME", "NODE", "CPU(m)", "MEM(Mi)")
	shc(`kubectl top pods -n rook-ceph | grep "osd" | sort -k2 | awk '{printf "%-20s | %-10s | %-10s | %-10s\n", $1, "Node_Ref", $2, $3}'`)

	// 4. Verificação de DB/WAL Expansion (RocksDB Pressure)
	fmt.Print("\n[4] ALERTAS DE ROCKSDB / EXPANSÃO\n")
	fmt.Println("------------------------------------------------------------------------")
	osd2Node := shcOut(`kubectl get pod -n rook-ceph -l ceph-osd-id=2 -o jsonpath='{.items[0].spec.nodeName}'`)
	fmt.Println("Dica: O OSD 2 no nó " + osd2Node + " detectou expansão de 436 GiB.")
	fmt.Println("Verifique o dmesg no host para erros de I/O: 'ssh " + osd2Node + " dmesg -T | grep -i sdb'")

	fmt.Print("\n========================================================================\n")
}

// --- fim do arquivo polyglot (ceph_osd.go/ceph_osd.sh) ---

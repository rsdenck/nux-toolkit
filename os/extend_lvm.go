/* 2>/dev/null
# ---------------------------------------------------------------------------
#  extend_lvm.sh — polyglot: este mesmo arquivo é extend_lvm.go (Go) e extend_lvm.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run extend_lvm.go [argumentos]
#    Bash ...: bash extend_lvm.sh  [argumentos]   (exige a toolchain Go)
#
#  Projeto: GETDEV
#  Script: extend_lvm.sh
#  Description: LVM Disk Expansion - Interactive Menu
#  Author: ranlens.denck
#  Category: os
#  Compatibility: Ubuntu, Debian, RHEL, Rocky, Alma, CentOS, Arch
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[extend_lvm.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[extend_lvm.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-extend_lvm"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/extend_lvm.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init extend_lvm >/dev/null 2>&1) || :

_bin="$_trab/extend_lvm"
if [ ! -x "$_bin" ] || [ "$_trab/extend_lvm.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando extend_lvm.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o extend_lvm.tmp extend_lvm.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o extend_lvm.tmp extend_lvm.go) ||
			_die "falha ao compilar $_trab/extend_lvm.go"
	}
	mv -f "$_trab/extend_lvm.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (extend_lvm.go/extend_lvm.sh) ---'
*/
// extend_lvm.go — expansão de disco via LVM com menu interativo.
//
// Porte fiel do script original extend_lvm.sh (GETDEV): coleta inventário
// (findmnt, vgs/lvs/pvs, lsblk) e oferece menu para estender VG (pvcreate +
// vgextend), estender LV (lvextend -l +100%FREE + grow do filesystem,
// detectando xfs via `mount | grep`) e rescan manual do dispositivo. Cada
// operação passa por confirm() que exige digitar literalmente "yes".
// Nenhum texto de saída foi traduzido ou alterado.
//
// Quirks fiéis ao original (mantidos de propósito):
//
//   - mkdir -p /var/log/extend-lvm roda no topo, ANTES da checagem de root
//     (efeito colateral: o diretório é tentado mesmo quando o usuário aborta);
//   - o help anuncia --dry-run, que não existe no case: argumento desconhecido
//     cai no menu interativo (--apply|*), sem modo dry-run;
//   - MODE="interactive" é atribuído e nunca lido no original — omitido aqui.
//
// Execução:
//
//	go run extend_lvm.go [--show|--apply|--help]
//	bash extend_lvm.sh  [--show|--apply|--help]  (mesmo arquivo, polyglot Go + sh)
package main

import (
	"bufio"
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
	cReset   = "\033[0m"
	cVerde   = "\033[0;32m"
	cVerm    = "\033[0;31m"
	cAmarelo = "\033[1;33m"
)

func pintar(code, s string) string {
	if !corAtiva {
		return s
	}
	return code + s + cReset
}

// cor devolve apenas o código de cor, sem o reset — usado no banner do
// original, onde o GREEN abre na primeira linha e o NC só fecha na última.
func cor(code string) string {
	if !corAtiva {
		return ""
	}
	return code
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

func logWarn(format string, a ...interface{}) {
	fmt.Println(pintar(cAmarelo, "[WARN]") + " " + fmt.Sprintf(format, a...))
}

func logErr(format string, a ...interface{}) {
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
	logErr("%v", err)
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

// shc roda um trecho de shell verbatim; falha não tratada encerra o programa
// como faria set -e.
func shc(script string) {
	if err := shEnv(script); err != nil {
		morrer(err)
	}
}

// comandoSaida roda um comando capturando o stdout (equivale a $(...) do
// original); falha encerra o programa como faria set -e.
func comandoSaida(nome string, args ...string) string {
	cmd := exec.Command(nome, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		morrer(err)
	}
	return string(out)
}

// ---------------------------------------------------------------------------
// Entrada (read do original via bufio; EOF cancela com código 1)
// ---------------------------------------------------------------------------

var leitor = bufio.NewReader(os.Stdin)

func stdinTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// ler equivale ao `read -rp` do original (IFS=$'\n\t'): raw preserva as
// barras invertidas (-r) e o recorte final remove apenas tabulas — espaços
// são preservados, como no original. O prompt vai para o stderr apenas quando
// o stdin é terminal (como o bash faz). EOF cancela a execução (exit 1).
func ler(prompt string) string {
	if stdinTTY() {
		fmt.Fprint(os.Stderr, prompt)
	}
	ln, err := leitor.ReadString('\n')
	if err != nil {
		os.Exit(1)
	}
	return strings.Trim(strings.TrimSuffix(ln, "\n"), "\t")
}

// ---------------------------------------------------------------------------
// Funções do original
// ---------------------------------------------------------------------------

// Header
func header() {
	shc("clear")
	fmt.Println(cor(cVerde) + "===============================================")
	fmt.Println("   LVM DISK EXPANSION - INTERACTIVE MENU")
	fmt.Println("   Author: ranlens.denck")
	fmt.Println("===============================================" + cor(cReset))
	fmt.Println()
}

func pause() {
	ler("Press ENTER to continue...")
}

func confirm() bool {
	ans := ler("WARNING: CONFIRM this operation? (yes/NO): ")
	return ans == "yes"
}

// Environment detection
var rootSrc, rootFS, rootType string

func detectRootLayout() {
	rootSrc = strings.TrimSpace(comandoSaida("findmnt", "-n", "-o", "SOURCE", "/"))
	rootFS = strings.TrimSpace(comandoSaida("findmnt", "-n", "-o", "FSTYPE", "/"))

	// [[ "$ROOT_SRC" =~ mapper ]] — regex literal sem meta, equivalente a Contains.
	if strings.Contains(rootSrc, "mapper") {
		rootType = "LVM"
	} else {
		rootType = "DIRECT PARTITION"
	}
}

func showInventory() {
	header()
	detectRootLayout()

	logInfo("Filesystem for /:")
	fmt.Printf("-> %s (%s)\n", rootFS, rootType)
	fmt.Println()

	logInfo("Volume Groups (VGs):")
	shc("vgs --units g")
	fmt.Println()

	logInfo("Logical Volumes (LVs):")
	shc("lvs --units g")
	fmt.Println()

	logInfo("Physical Volumes (PVs):")
	shc("pvs --units g")
	fmt.Println()

	logInfo("Block Devices:")
	shc("lsblk -o NAME,TYPE,SIZE,FSTYPE,MOUNTPOINT")
	fmt.Println()
}

// Execution logic (Simplified for brevity but functional)
func expandVG() {
	dev := ler("Enter Device to add to VG (e.g. /dev/sdb): ")
	vgname := ler("Enter VG Name: ")
	if confirm() {
		executar("pvcreate", dev)
		executar("vgextend", vgname, dev)
		logInfo("VG %s extended.", vgname)
	}
}

func expandLV() {
	lvpath := ler("Enter LV Path (e.g. /dev/mapper/vg0-root): ")
	if confirm() {
		executar("lvextend", "-l", "+100%FREE", lvpath)
		// Pipeline original `mount | grep "$LVPATH" | grep -q "xfs"` verbatim.
		if shEnv(`mount | grep "$LVPATH" | grep -q "xfs"`, "LVPATH="+lvpath) == nil {
			executar("xfs_growfs", lvpath)
		} else {
			executar("resize2fs", lvpath)
		}
		// "%%" preserva o literal "100% free space." do original (o vet lê "% f" como verb)
		logInfo("LV %s extended to 100%% free space.", lvpath)
	}
}

// ---------------------------------------------------------------------------
// Ajuda e aplicação
// ---------------------------------------------------------------------------

func printHelp() {
	fmt.Print(`Usage: extend_lvm.sh [--show|--dry-run|--apply|--help]

--show     : Collect inventory and recommendations
--dry-run  : Show commands that would be executed (no execution)
--apply    : Execute selected operations (interactive menu)
--help     : This help
`)
}

func main() {
	prepararSaida()

	// Efeito colateral fiel: no original o LOGDIR é criado no topo, ANTES da
	// checagem de root (mkdir falho também encerra com exit 1, via set -e).
	shc("mkdir -p /var/log/extend-lvm")

	// Main: Check root
	if os.Geteuid() != 0 {
		logErr("This script must be run as root.")
		os.Exit(1)
	}

	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}
	switch arg {
	case "--show":
		showInventory()
		os.Exit(0)
	case "--help":
		printHelp()
		os.Exit(0)
	default:
		// --apply|* — fiel: qualquer outro argumento (inclusive --dry-run,
		// citado apenas no help) cai no menu, sem modo dry-run.
		for {
			showInventory()
			fmt.Println("MENU:")
			fmt.Println("1) Extend VG (Add new PV)")
			fmt.Println("2) Extend LV (to 100% VG free space)")
			fmt.Println("3) Manual Rescan")
			fmt.Println("0) Exit")
			fmt.Println()
			opt := ler("Select an option: ")
			switch opt {
			case "1":
				expandVG()
			case "2":
				expandLV()
			case "3":
				dev := ler("Enter device (e.g. sdb): ")
				// Com "|| ..." no original: falha aqui apenas loga e continua.
				if err := shEnv(`echo 1 > "/sys/class/block/$DEV/device/rescan"`, "DEV="+dev); err != nil {
					logErr("Rescan failed.")
				}
			case "0":
				os.Exit(0)
			default:
				logWarn("Invalid option.")
			}
			pause()
		}
	}
}

// --- fim do arquivo polyglot (extend_lvm.go/extend_lvm.sh) ---

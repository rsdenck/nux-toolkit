/* 2>/dev/null
# ---------------------------------------------------------------------------
#  extend_disk.sh — polyglot: este mesmo arquivo é extend_disk.go (Go) e extend_disk.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run extend_disk.go [argumentos]
#    Bash ...: bash extend_disk.sh  [argumentos]   (exige a toolchain Go)
#
#  Projeto: GETDEV
#  Script: extend_disk.sh
#  Description: Safe expansion of Disk, Partition, and Filesystem
#  Author: ranlens.denck
#  Category: os
#  Compatibility: Ubuntu, Debian, RHEL, Rocky, Alma, CentOS, Arch
#  Supports: ext4 / xfs, Simple disk or LVM, NVMe / multipath
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[extend_disk.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[extend_disk.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-extend_disk"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/extend_disk.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init extend_disk >/dev/null 2>&1) || :

_bin="$_trab/extend_disk"
if [ ! -x "$_bin" ] || [ "$_trab/extend_disk.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando extend_disk.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o extend_disk.tmp extend_disk.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o extend_disk.tmp extend_disk.go) ||
			_die "falha ao compilar $_trab/extend_disk.go"
	}
	mv -f "$_trab/extend_disk.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (extend_disk.go/extend_disk.sh) ---'
*/
// extend_disk.go — expansão segura de disco, partição e filesystem.
//
// Porte fiel do script original extend_disk.sh (GETDEV): exibe o inventário
// (lsblk) e oferece menu interativo com rescan manual do disco (sysfs),
// expansão de partição (growpart) e expansão de filesystem (detectando xfs
// via `mount | grep` → xfs_growfs, senão resize2fs). Suporta --show, --dry-run
// e --apply. Nenhum texto de saída foi traduzido ou alterado.
//
// Fiel ao original, a checagem de root acontece ANTES do parse de argumentos
// (até --help exige root).
//
// Execução:
//
//	go run extend_disk.go [--show|--dry-run|--apply|--help]
//	bash extend_disk.sh  [--show|--dry-run|--apply|--help]  (mesmo arquivo, polyglot Go + sh)
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

// shEnv roda um trecho de shell verbatim do original (pipelines,
// redirecionamentos, &&) com variáveis extras no ambiente.
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

// ---------------------------------------------------------------------------
// Entrada (read do original via bufio; EOF cancela com código 1)
// ---------------------------------------------------------------------------

var leitor = bufio.NewReader(os.Stdin)

func stdinTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// removerEscapes comporta-se como o read do bash sem -r: a barra invertida
// escapa o caractere seguinte e "\nova linha" é continuação de linha.
func removerEscapes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			if i+1 < len(s) {
				if s[i+1] != '\n' {
					b.WriteByte(s[i+1])
				}
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// ler equivale ao `read -p` do original: o prompt vai para o stderr apenas
// quando o stdin é terminal (como o bash faz); com raw=false as barras
// invertidas agem como escape (read sem -r). EOF cancela a execução (exit 1).
// O recorte final segue o IFS padrão do original (espaços e tabulas).
func ler(prompt string, raw bool) string {
	if stdinTTY() {
		fmt.Fprint(os.Stderr, prompt)
	}
	ln, err := leitor.ReadString('\n')
	if err != nil {
		os.Exit(1)
	}
	ln = strings.TrimSuffix(ln, "\n")
	if !raw {
		ln = removerEscapes(ln)
	}
	return strings.Trim(ln, " \t")
}

// lerDois equivale ao `read -p "..." DEV PART` do original (IFS padrão: os
// campos são separados por espaços/tab e o último leva o resto da linha).
func lerDois(prompt string) (string, string) {
	campos := strings.Fields(ler(prompt, false))
	if len(campos) == 0 {
		return "", ""
	}
	return campos[0], strings.Join(campos[1:], " ")
}

// ---------------------------------------------------------------------------
// Executor com dry-run (equivalente ao run()/eval do original)
// ---------------------------------------------------------------------------

type app struct {
	dryRun bool
}

// run equivale ao run() do original: em modo dry-run apenas anuncia o
// comando; caso contrário ele é avaliado no shell (aqui via sh -c verbatim —
// o original usava eval "$@").
func (a *app) run(comando string) error {
	if a.dryRun {
		fmt.Println(pintar(cAmarelo, "[DRY-RUN]") + " " + comando)
		return nil
	}
	return shEnv(comando)
}

// ehXFS roda o pipeline original `mount | grep "$TARGET" | grep -q "xfs"`
// verbatim, com o alvo exportado como variável de ambiente (como no original).
func ehXFS(alvo string) bool {
	return shEnv(`mount | grep "$TARGET" | grep -q "xfs"`, "TARGET="+alvo) == nil
}

// ---------------------------------------------------------------------------
// Ajuda e cabeçalho
// ---------------------------------------------------------------------------

func printHelp() {
	fmt.Print(`Usage: extend_disk.sh [--show|--dry-run|--apply|--help]

--show     : Display complete inventory
--dry-run  : Simulate operations (no execution)
--apply    : Execute operations
--help     : This help
`)
}

func header() {
	shc("clear")
	fmt.Println(cor(cVerde) + "===============================================")
	fmt.Println("   DISK, PARTITION AND FILESYSTEM EXPANSION")
	fmt.Println("   Author: ranlens.denck")
	fmt.Println("===============================================" + cor(cReset))
	fmt.Println()
}

// ---------------------------------------------------------------------------
// Aplicação
// ---------------------------------------------------------------------------

func main() {
	prepararSaida()
	a := &app{}

	// Check root — no original ANTES do parse de argumentos (fiel: até --help
	// exige root).
	if os.Geteuid() != 0 {
		logErr("This script must be run as root.")
		os.Exit(1)
	}

	// Parsing arguments
	mode := "menu"
	arg := ""
	if len(os.Args) > 1 {
		arg = os.Args[1]
	}
	switch arg {
	case "--show":
		mode = "show"
	case "--dry-run":
		mode = "menu"
		a.dryRun = true
	case "--apply":
		mode = "menu"
	case "--help":
		printHelp()
		os.Exit(0)
	default:
		mode = "menu"
	}

	// Header
	header()

	// Inventory
	logInfo("Disks:")
	shc(`lsblk -d -o NAME,TYPE,SIZE | grep -E 'disk|mpath|nvme' || true`)
	fmt.Println()

	logInfo("Structure:")
	shc("lsblk -o NAME,TYPE,SIZE,FSTYPE,MOUNTPOINT")
	fmt.Println()

	if mode == "show" {
		os.Exit(0)
	}

	// Menu
	for {
		fmt.Println("MENU:")
		fmt.Println("1) Manual disk rescan")
		fmt.Println("2) Expand partition")
		fmt.Println("3) Expand filesystem")
		fmt.Println("0) Exit")
		fmt.Println()
		opt := ler("Select an option: ", false)
		switch opt {
		case "1":
			dev := ler("Enter device name (e.g. sda, nvme0n1): ", false)
			if err := a.run(fmt.Sprintf("echo 1 > /sys/class/block/%s/device/rescan", dev)); err != nil {
				logErr("Rescan failed.")
			}
		case "2":
			dev, part := lerDois("Enter device and partition (e.g. /dev/sda 1): ")
			if err := a.run(fmt.Sprintf("growpart %s %s", dev, part)); err != nil {
				logErr("Growpart failed.")
			}
		case "3":
			alvo := ler("Enter mountpoint or device (e.g. / or /dev/sda1): ", false)
			// Sem "|| ..." no original: falha aqui encerra o script (set -e).
			if ehXFS(alvo) {
				if err := a.run("xfs_growfs " + alvo); err != nil {
					morrer(err)
				}
			} else {
				if err := a.run("resize2fs " + alvo); err != nil {
					morrer(err)
				}
			}
		case "0":
			os.Exit(0)
		default:
			logWarn("Invalid option.")
		}
	}
}

// --- fim do arquivo polyglot (extend_disk.go/extend_disk.sh) ---

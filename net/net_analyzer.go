/* 2>/dev/null
# ---------------------------------------------------------------------------
#  net_analyzer.sh — polyglot: este mesmo arquivo é net_analyzer.go (Go) e
#  net_analyzer.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — a fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run net_analyzer.go [argumentos]
#    Bash ...: bash net_analyzer.sh  [argumentos]   (exige a toolchain Go)
#
#  Project:     GETDEV
#  Script:      net_analyzer.sh
#  Description: Professional Network Analyzer & Speed Test (TUI)
#  Author:      GETDEV Team
#  Category:    Network
#  Compatibility: Ubuntu, Debian, RHEL, Rocky, Alma, Fedora, Arch
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[net_analyzer.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[net_analyzer.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-net_analyzer"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/net_analyzer.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init net_analyzer >/dev/null 2>&1) || :

_bin="$_trab/net_analyzer"
if [ ! -x "$_bin" ] || [ "$_trab/net_analyzer.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando net_analyzer.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o net_analyzer.tmp net_analyzer.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o net_analyzer.tmp net_analyzer.go) ||
			_die "falha ao compilar $_trab/net_analyzer.go"
	}
	mv -f "$_trab/net_analyzer.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export NUX_VIA_BASH=1
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (net_analyzer.go/net_analyzer.sh) ---'
*/
// net_analyzer.go — analisador de rede profissional com teste de velocidade (TUI).
//
// Porta fiel do script original net_analyzer.sh (GETDEV): exibe as interfaces
// de rede (ip -br addr show) coloridas, o IP público e a geo-localização via
// ipapi.co (extração com grep -oP, sem jq) e um teste de latência (ping
// 1.1.1.1) + download de 10 MB via speed.cloudflare.com. Nenhum texto de
// saída foi traduzido ou alterado.
//
// Execução:
//
//	go run net_analyzer.go
//	bash net_analyzer.sh   (mesmo arquivo, polyglot Go + sh)
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Saída (mesmas cores e labels do original; cor só em TTY e sem NO_COLOR)
// ---------------------------------------------------------------------------

var corAtiva bool

const (
	cReset = "\033[0m"
	cVerde = "\033[0;32m"
	cCiano = "\033[0;36m"
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

// logStat espelha o original (log_stat usa apenas $1, sem formatação).
func logStat(msg string) {
	fmt.Println("  " + pintar(cCiano, ">") + " " + msg)
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
	fmt.Println(pintar(cVerm, "[ERROR]") + " " + err.Error())
	os.Exit(1)
}

// shcOut roda um pipeline verbatim do original e devolve o stdout (como em
// $(...)); falha não tratada encerra o programa (set -e).
func shcOut(script string) string {
	cmd := exec.Command("sh", "-c", script)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		morrer(err)
	}
	return buf.String()
}

// shPipe roda um pipeline verbatim do original com o pipefail do set -o
// pipefail (usa bash quando disponível, senão sh) e devolve o stdout.
func shPipe(script string) string {
	var cmd *exec.Cmd
	if _, err := exec.LookPath("bash"); err == nil {
		cmd = exec.Command("bash", "-c", "set -o pipefail; "+script)
	} else {
		cmd = exec.Command("sh", "-c", script)
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = os.Stderr
	_ = cmd.Run() // o "|| echo" do próprio pipeline já cobre a falha
	return strings.TrimRight(buf.String(), "\n")
}

// ---------------------------------------------------------------------------
// Header
// ---------------------------------------------------------------------------

func header() {
	run("clear")
	fmt.Println(pintar(cVerde, "===================================================="))
	fmt.Println(pintar(cVerde, "         GETDEV NETWORK ANALYZER (TUI)              "))
	fmt.Println(pintar(cVerde, "===================================================="))
}

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

func analyzeInterfaces() {
	logInfo("Analyzing Network Interfaces...")
	fmt.Printf("  %-15s %-10s %-25s\n", "INTERFACE", "STATUS", "ADDRESSES")
	fmt.Println("  --------------------------------------------------")
	out := shcOut("ip -br addr show")
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		// ifname/ips como no original: awk '{print $1}' e cut -d' ' -f3-
		partes := strings.Split(line, " ")
		var ifname, status, ips string
		if len(partes) > 0 {
			ifname = partes[0]
		}
		if len(partes) > 1 {
			status = partes[1]
		}
		if len(partes) > 2 {
			ips = strings.Join(partes[2:], " ")
		}
		// status colorido: UP/UNKNOWN em verde, demais em vermelho
		statusColor := pintar(cVerde, status)
		if status != "UP" && status != "UNKNOWN" {
			statusColor = pintar(cVerm, status)
		}
		fmt.Printf("  %-15s %-18s %-25s\n", ifname, statusColor, ips)
	}
	fmt.Println()
}

// ---------------------------------------------------------------------------
// IP público e geo-localização
// ---------------------------------------------------------------------------

func geoLocation() {
	logInfo("Public IP & Geo-location:")
	geoData := strings.TrimRight(shPipe(`curl -s https://ipapi.co/json/ || echo "error"`), "\n")

	if geoData == "error" || strings.Contains(geoData, "error") {
		logStat("Could not retrieve geo-location data (Service unavailable).")
	} else {
		// extração manual para não depender de 'jq' — mesmo grep -oP do original
		ip := extrairCampo(geoData, "ip")
		city := extrairCampo(geoData, "city")
		region := extrairCampo(geoData, "region")
		country := extrairCampo(geoData, "country_name")
		org := extrairCampo(geoData, "org")

		logStat("IP Address:   " + ip)
		logStat("Location:     " + city + ", " + region + ", " + country)
		logStat("Organization: " + org)
	}
	fmt.Println()
}

// extrairCampo roda o pipeline original via sh -c:
// echo "$geo_data" | grep -oP '(?<="chave": ")[^"]*' || echo "N/A"
func extrairCampo(geoData, chave string) string {
	padrao := `(?<="` + chave + `": ")[^"]*`
	cmd := exec.Command("sh", "-c", `echo "$1" | grep -oP '`+padrao+`' || echo "N/A"`, "sh", geoData)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	_ = cmd.Run() // o "|| echo N/A" do próprio pipeline já cobre a falha
	return strings.TrimRight(buf.String(), "\n")
}

// ---------------------------------------------------------------------------
// Speed test
// ---------------------------------------------------------------------------

func speedTest() {
	logInfo("Professional Speed Test (Custom Engine):")

	// 1. Latency Check
	logStat("Testing Latency (Cloudflare)...")
	pingVal := shPipe(`ping -c 4 1.1.1.1 2>/dev/null | tail -1 | awk -F '/' '{print $5}' || echo "N/A"`)
	logStat("Latency: " + pingVal + " ms")

	// 2. Download Test
	logStat("Testing Download Speed (10MB Sample)...")
	speedBPS := shPipe(`curl -L -s -w "%{speed_download}" -o /dev/null "https://speed.cloudflare.com/__down?bytes=10485760" || echo "0"`)

	if speedBPS == "0" {
		logStat("Download Speed: Failed to measure.")
	} else {
		// Mbps = bytes/s * 8 / 1000000 — mesmo cálculo e formatação (%.6g) do awk
		v, err := strconv.ParseFloat(strings.TrimSpace(speedBPS), 64)
		if err != nil {
			v = 0 // awk trataria a string não numérica como 0
		}
		mbps := strconv.FormatFloat(v*8/1000000, 'g', 6, 64)
		logStat("Download Speed: " + mbps + " Mbps")
	}
}

func main() {
	prepararSaida()

	// Check for curl
	if _, err := exec.LookPath("curl"); err != nil {
		fmt.Println(pintar(cVerm, "[ERROR]") + " curl is required but not installed. Exiting.")
		os.Exit(1)
	}

	header()
	analyzeInterfaces()
	geoLocation()
	speedTest()
	fmt.Println()
	fmt.Println(pintar(cVerde, "===================================================="))
}

// --- fim do arquivo polyglot (net_analyzer.go/net_analyzer.sh) ---

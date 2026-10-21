/* 2>/dev/null
# ---------------------------------------------------------------------------
#  lunar.sh — polyglot: este mesmo arquivo é lunar.go (Go) e lunar.sh (Bash/sh).
#  O trecho abaixo apenas compila e executa a implementação Go — fonte única
#  da ferramenta. Nada é duplicado e nenhuma ferramenta extra é instalada.
#
#    Go .....: go run lunar.go [argumentos]
#    Bash ...: bash lunar.sh  [argumentos]   (exige a toolchain Go)
#
#  Cabeçalho do script original:
# -----------------------------------------------------------------------------
# Project:     GETDEV
# Script:      lunar.sh
# Description: Lockdown UNix Auditing and Reporting (Standardized)
# Version:     10.9.1
# Author:      GETDEV Team
# Category:    Security / Auditing
# Compatibility: Multi-distribution (Debian, RHEL, Solaris, etc.)
# -----------------------------------------------------------------------------
#
# No warranty is implied or given with this script
# It is based on numerous security guidelines
# As with any system changes, the script should be vetted and
# changed to suit the environment in which it is being used
#
# Unless your organization is specifically using the service, disable it.
# The best defense against a service being exploited is to disable it.
#
# Even if a service is set to off the script will audit the configuration
# file so that if a service is re-enabled the configuration is secure
# Where possible checks are made to make sure the package is installed
# if the package is not installed the checks will not be run
#
# To do:
#
# - nosuid,noexec for Linux
# - Disable user mounted removable filesystems for Linux
# - Disable USB devices for Linux
# - Grub password
# - Restrict NFS client requests to privileged ports Linux
#
# Solaris Release Information
#  1/06 U1
#  6/06 U2
# 11/06 U3
#  8/07 U4
#  5/08 U5
# 10/08 U6
#  5/09 U7
# 10/09 U8
#  9/10 U9
#  8/11 U10
#  1/13 U11
#
# audit_mode = 1 : Audit Mode
# audit_mode = 0 : Lockdown Mode
# audit_mode = 2 : Restore Mode
# ---------------------------------------------------------------------------
_self="$0"

# cores apenas em terminal e sem NO_COLOR
_ci=""; _cf=""; _cr=""
if [ -t 1 ] && [ -z "${NO_COLOR-}" ]; then _ci='\033[1;36m'; _cf='\033[1;31m'; _cr='\033[0m'; fi

_info() { printf '%s[lunar.sh]%s %s\n' "$_ci" "$_cr" "$*"; }
_erro() { printf '%s[lunar.sh]%s %s\n' "$_cf" "$_cr" "$*" >&2; }
_die() { _erro "$*"; exit 1; }

command -v go >/dev/null 2>&1 || _die "toolchain Go (comando 'go') não encontrada — instale em https://go.dev/dl"

_trab="${TMPDIR:-/tmp}/nux-lunar"
mkdir -p "$_trab" || _die "sem permissão para escrever em $_trab"
cp -f -- "$_self" "$_trab/lunar.go" || _die "não consegui copiar $_self"
[ -f "$_trab/go.mod" ] || (cd "$_trab" && go mod init lunar >/dev/null 2>&1) || :

_bin="$_trab/lunar"
if [ ! -x "$_bin" ] || [ "$_trab/lunar.go" -nt "$_bin" ] || [ "$_self" -nt "$_bin" ]; then
	_info "compilando lunar.go (primeira execução ou versão atualizada)..."
	(cd "$_trab" && go build -o lunar.tmp lunar.go) || {
		(export GO111MODULE=on; cd "$_trab" && go build -o lunar.tmp lunar.go) ||
			_die "falha ao compilar $_trab/lunar.go"
	}
	mv -f "$_trab/lunar.tmp" "$_bin" || _die "não consegui gravar $_bin"
	chmod 755 "$_bin" 2>/dev/null || :
fi

export LUNAR_VIA_BASH=1
# o Go usa o caminho real do script ($0) para "$0 may need root", dirname,
# scp -r do diretório do script e para extrair "# Version:" do próprio arquivo
export LUNAR_SRC="$_self"
exec "$_bin" "$@"
exit 1

# O heredoc abaixo esconde o código Go dos parsers estáticos de shell
# (ex.: bash -n). Em execução normal o "exit 1" acima já encerrou o shell.
: <<'// --- fim do arquivo polyglot (lunar.go/lunar.sh) ---'
*/
// lunar.go — Lockdown UNix Auditing and Reporting (Standardized), GETDEV v10.9.1.
//
// Porte fiel do script lunar.sh: audita (audit mode, sem mudanças), aplica
// endurecimento (lockdown mode, com mudanças) e restaura configurações
// (restore mode) em sistemas Unix-like, com suporte a execução em
// docker-compose, multipass e hosts remotos (scp + ssh).
//
// As funções de auditoria (funct_audit_*), a ajuda (print_help) e demais
// utilitários eram carregados do diretório core/ do projeto original, que não
// faz parte deste repositório: no porte, cada chamada a essas funções
// reproduz o comportamento do bash sem set -e — imprime
// "<função>: command not found" em stderr e o fluxo continua. Da mesma forma,
// falhas de comandos externos não abortam a execução.
//
// Execução:
//
//	go run lunar.go [argumentos]
//	bash lunar.sh  [argumentos]
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Colors for output (sequências ANSI do original; emitidas apenas com
// terminal e sem NO_COLOR)
// ---------------------------------------------------------------------------

const (
	greenANSI = "\033[0;32m"
	redANSI   = "\033[0;31m"
	ncANSI    = "\033[0m" // No Color
)

var colorOn bool

// ---------------------------------------------------------------------------
// Defaults for AWS
// ---------------------------------------------------------------------------

var (
	aws_iam_master_role      = "iam-master"
	aws_iam_manager_role     = "iam-manager"
	aws_cloud_trail_name     = "aws-audit-log"
	sns_protocol             = "email"
	sns_endpoint             = "alerts@company.com"
	valid_tag_string         = `(ue1|uw1|uw2|ew1|ec1|an1|an2|as1|as2|se1)-(d|t|s|p)-([a-z0-9\-]+)$`
	strict_valid_names       = "y"
	check_volattach          = "y"
	check_voltype            = "y"
	check_snapage            = "y"
	aws_region               = ""
	aws_rds_min_retention    = "7"
	aws_ec2_min_retention    = "7"
	aws_ec2_max_retention    = "30"
	aws_days_to_key_deletion = "7"
)

// ---------------------------------------------------------------------------
// Defaults for MacOS
// ---------------------------------------------------------------------------

var (
	keychain_sync   = "1"
	disable_airdrop = "1"
	asset_cache     = "false"
	wifi_status     = "2"
	touchid_timeout = "86400"
)

// ---------------------------------------------------------------------------
// Defaults for SSH
// ---------------------------------------------------------------------------

var (
	ssh_protocol    = "2"
	ssh_key_size    = "4096"
	ssh_allowusers  = ""
	ssh_allowgroups = ""
	ssh_denyusers   = ""
	ssh_denygroups  = ""
)

// ---------------------------------------------------------------------------
// Set up some counters
// ---------------------------------------------------------------------------

var (
	secure_count   = 0
	insecure_count = 0
	lockdown_count = 0
	restore_count  = 0
	total_count    = 0
)

// ---------------------------------------------------------------------------
// Set up some global variables/defaults
// (o original não usa `local`: todas as atribuições em funções são globais)
// ---------------------------------------------------------------------------

var (
	app_dir           string // dirname "$0" (calculado no main)
	args              string // "$@" (sem uso no original)
	use_sudo          = 0
	syslog_server     = ""
	syslog_logdir     = ""
	pkg_suffix        = "lunar"
	base_dir          = "/var/log/" + pkg_suffix
	temp_dir          = base_dir + "/tmp"
	date_suffix       string // date +%d_%m_%Y_%H_%M_%S
	temp_file         string // "${temp_dir}/${pkg_suffix}.tmp"
	work_dir          string // "${base_dir}/${date_suffix}"
	wheel_group       = "wheel"
	docker_group      = "docker"
	reboot_required   = 0
	verbose_mode      = 0
	dryrun_mode       = 0
	ansible_mode      = 0
	ansible_counter   = 0
	core_dir          string // "${app_dir}/core"
	functions_dir     string // "${app_dir}/functions"
	modules_dir       string // "${app_dir}/modules"
	private_dir       string // "${app_dir}/private"
	package_uninstall = "no"
	country_suffix    = "au"
	language_suffix   = "en_US"
	osx_mdns_enable   = "yes"
	max_super_user_id = "100"
	use_expr          = "no"
	use_finger        = "yes"
	test_os           = "none"
	test_tag          = "none"
	action            = "none"
	do_compose        = 0
	do_multipass      = 0
	do_shell          = 0
	do_remote         = 0
	my_id             string // id -u
	tcpd_allow        = "sshd"
	do_audit          = 0
	do_fs             = 0
	audit_mode        = 1
	audit_type        = "local"
	git_url           = "https://github.com/lateralblast/lunar.git"
	password_hashing  = "sha512"
	anacron_enable    = "no"
	ssh_sandbox       = "yes"
	do_debug          = 0
	do_select         = 0
	module_name       = ""
	no_cat            = 0
	ubuntu_codename   = ""
)

// Disable daemons

var (
	nfsd_disable     = "yes"
	snmpd_disable    = "yes"
	dhcpcd_disable   = "yes"
	dhcprd_disable   = "yes"
	dhcpsd_disable   = "yes"
	sendmail_disable = "yes"
	ipv6_disable     = "yes"
	routed_disable   = "yes"
	named_disable    = "yes"
)

// Variáveis de tempo de execução (identificadas no script original)

var (
	os_name        string // uname
	id_check       string // id -u (ou `id | cut -c5` no SunOS); fica vazia em VMkernel
	arg_test       string // echo "$@" | grep -cE ...
	start_path     string // pwd
	script_version string // grep '^# Version:' < "$0" | awk '{print $3}'
	// variáveis de linha de comando (criadas no parser; não inicializadas no
	// original, portanto vazias)
	test_distro   string
	ext_host      string
	module        string
	restore_date  string
	restore_dir   string
	vm_type       string
	do_kubernetes = 0
	audit_select  = 0
	force         = 0
	do_aws        = 0
	do_aws_rec    = 0
	do_docker     = 0
	print_funct   = 0
)

// Variáveis que o original declara sem `local` (portanto globais)

var (
	text            string // verbose_message
	style           string // verbose_message
	message         string // warning_message / execute_* / increment_*
	command         string // execute_lockdown / execute_restore (lida por restore_state)
	privilege       string // execute_lockdown / execute_restore
	virtual         string // check_virtual_platform
	dmi_check       string // check_virtual_platform
	aws_bin         string // check_aws
	aws_creds       string // check_aws
	base64_d        string // check_aws
	backup_name     string
	backup_value    string
	backup_file     string
	restore_name    string
	current_value   string
	restore_command string
	restore_file    string
	restore_value   string
)

// ---------------------------------------------------------------------------
// Logging functions
// ---------------------------------------------------------------------------

func log_info(text string) {
	printStyled(greenANSI, "[INFO]", " ", text)
}

func log_error(text string) {
	printStyled(redANSI, "[ERROR]", " ", text)
}

// ---------------------------------------------------------------------------
// warning_message
//
// Warning message
//.

func warning_message(p1 string) {
	message = p1
	verbose_message(message, "warn")
}

// ---------------------------------------------------------------------------
// lockdown_warning
//
// Check whether to proceed
//.

func lockdown_warning() {
	if dryrun_mode == 0 {
		if force != 1 {
			warning_message("This will alter the system")
			fmt.Printf("%s", "Do you want to continue? (yes/no): ")
			for {
				reply, err := readStdinLine()
				if err != nil {
					// stdin fechado: no original o `read` falha, o laço termina e o
					// script prosseguiria; conforme a regra 4 do guia de porte,
					// EOF no stdin cancela com código 1
					os.Exit(1)
				}
				switch reply {
				case "yes":
					return
				default:
					// `exit` sem argumento: o último comando (read) teve status 0
					os.Exit(0)
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// verbose_message
//
// Print a message if verbose mode enabled
//.

func verbose_message(p1, p2 string) {
	text = p1
	style = p2
	if verbose_mode == 1 && style == "fix" {
		if text == "" {
			fmt.Println()
		} else {
			printStyled(greenANSI, "[ FIX ]", "     ", text)
		}
		return
	}
	switch style {
	case "audit", "auditing":
		printStyled(greenANSI, "[ AUDIT ]", "   ", text)
	case "load", "loading":
		printStyled(greenANSI, "[ LOAD ]", "    ", text)
	case "exec", "execute", "executing":
		printStyled(greenANSI, "[ EXEC ]", "    ", text)
	case "notice":
		printStyled(greenANSI, "[ NOTICE ]", "  ", text)
	case "delete", "deleting":
		printStyled(redANSI, "[ DELETE ]", "  ", text)
	case "install", "installing":
		printStyled(greenANSI, "[ INSTALL ]", " ", text)
	case "backup":
		printStyled(greenANSI, "[ BACKUP ]", "  ", text)
	case "save", "saving":
		printStyled(greenANSI, "[ SAVE ]", "    ", text)
	case "set", "setting":
		printStyled(greenANSI, "[ SET ]", "     ", text)
	case "run", "running":
		printStyled(greenANSI, "[ RUN ]", "     ", text)
	case "restore", "restoring":
		printStyled(greenANSI, "[ RESTORE ]", " ", text)
	case "remove", "removing":
		printStyled(redANSI, "[ REMOVE ]", "  ", text)
	case "check", "checking":
		printStyled(greenANSI, "[ CHECK ]", "   ", text)
	case "create", "creating":
		printStyled(greenANSI, "[ CREATE ]", "  ", text)
	case "warn", "warning":
		printStyled(redANSI, "[ WARN ]", "    ", text)
	case "secure":
		printStyled(greenANSI, "[ SECURE ]", "  ", text)
	case "update":
		printStyled(greenANSI, "[ UPDATE ]", "  ", text)
	default:
		if verbose_mode == 1 {
			fmt.Println(text)
		}
	}
}

// ---------------------------------------------------------------------------
// cidr_to_mask
//
// Convert CIDR to netmask
//.

func cidr_to_mask(p1 string) string {
	// set -- $(( 5 - ($1 / 8) )) 255 255 255 255 $(( (255 << (8 - ($1 % 8))) & 255 )) 0 0 0
	n, err := strconv.Atoi(p1)
	if err != nil {
		n = 0 // no bash, valor não numérico vira 0 na aritmética
	}
	shift_n := 5 - n/8
	params := []string{
		strconv.Itoa(shift_n), "255", "255", "255", "255",
		strconv.Itoa((255 << (8 - n%8)) & 255), "0", "0", "0",
	}
	// if [ "$1" -gt 1 ]; then shift "$1" ; else shift ; fi
	if shift_n > 1 {
		if shift_n <= len(params) {
			params = params[shift_n:]
		} // shift acima do total falha no bash e os parâmetros não mudam
	} else {
		params = params[1:]
	}
	// echo "${1-0}"."${2-0}"."${3-0}"."${4-0}"
	get := func(k int) string {
		if k < len(params) {
			return params[k]
		}
		return "0"
	}
	return get(0) + "." + get(1) + "." + get(2) + "." + get(3)
}

// ---------------------------------------------------------------------------
// mask_to_cidr
//
// Convert netmask to CIDR
//.

func mask_to_cidr(p1 string) string {
	// x="${1##*255.}" — remove o prefixo mais longo terminando em "255."
	x := p1
	if idx := strings.LastIndex(p1, "255."); idx >= 0 {
		x = p1[idx+4:]
	}
	set := "0^^^128^192^224^240^248^252^254^"
	// set -- 0^^^128^... $(( (${#1} - ${#x})*2 )) "${x%%.*}"
	prefix := (len(p1) - len(x)) * 2
	third := x
	if idx := strings.Index(x, "."); idx >= 0 {
		third = x[:idx]
	}
	// x=${1%%"$3"*} — remove a partir da primeira ocorrência de "$3"
	y := set
	if idx := strings.Index(set, third); idx >= 0 {
		y = set[:idx]
	}
	// echo $(( $2 + (${#x}/4) ))
	return strconv.Itoa(prefix + len(y)/4)
}

// ---------------------------------------------------------------------------
// check_virtual_platform
//
// Check if we are running on a virtual platform
//.

func check_virtual_platform() {
	virtual = "Unknown"
	if _, err := os.Stat("/.dockerenv"); err == nil {
		virtual = "Docker"
	} else {
		dmi_check = shCapture(`command -v dmidecode | grep dmidecode | grep -cv no`)
		if dmi_check == "1" && my_id == "0" {
			virtual = shCapture(`dmidecode | grep Manufacturer |head -1 | awk '{print $2}' | sed "s/,//g"`)
		} else {
			virtual = execCapture("uname", "-p")
		}
	}
	fmt.Println("Platform:   " + virtual)
}

// ---------------------------------------------------------------------------
// execute_lockdown
//
// Run a lockdown command
// Check that we are in lockdown mode
// If not in lockdown mode output a verbose message
//.

func execute_lockdown(p1, p2, p3 string) {
	command = p1
	message = p2
	privilege = p3
	if audit_mode == 0 {
		total_count++
		if message != "" {
			verbose_message(message, "set")
		}
		verbose_message(command, "execute")
		if privilege == "" {
			if dryrun_mode == 0 {
				lockdown_count++
				shc(command)
			}
		} else {
			if my_id == "0" {
				if dryrun_mode == 0 {
					lockdown_count++
					shc(command)
				}
			} else {
				if use_sudo == 1 {
					if dryrun_mode == 0 {
						lockdown_count++
						sudoShc(command)
					}
				}
			}
		}
	} else {
		verbose_message(command, "fix")
	}
}

// ---------------------------------------------------------------------------
// execute_restore
//
// Run restore command
// Check we are running in restore mode run a command
//.

func execute_restore(p1, p2, p3 string) {
	command = p1
	message = p2
	privilege = p3
	if audit_mode == 2 {
		total_count++
		if message != "" {
			verbose_message(message, "restore")
		}
		verbose_message(command, "execute")
		if privilege == "" {
			if dryrun_mode == 0 {
				restore_count++
				shc(command)
			}
		} else {
			if my_id == "0" {
				if dryrun_mode == 0 {
					restore_count++
					shc(command)
				}
			} else {
				if use_sudo == 1 {
					if dryrun_mode == 0 {
						restore_count++
						sudoShc(command)
					}
				}
			}
		}
	} else {
		verbose_message(command, "fix")
	}
}

// ---------------------------------------------------------------------------
// backup_state
//
// Backup state to a log file for later restoration
//.

func backup_state(p1 string) {
	if audit_mode == 0 {
		backup_name = p1
		backup_value = p1
		backup_file = work_dir + "/" + backup_name + ".log"
		// echo "$backup_value" > "${backup_file}"
		if err := os.WriteFile(backup_file, []byte(backup_value+"\n"), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "lunar.sh: line 482: %s: %s\n", backup_file, bashErrText(err))
		}
	}
}

// ---------------------------------------------------------------------------
// restore_state
//
// Restore state from a log file
//.

func restore_state(p1, p2, p3 string) {
	if audit_mode == 2 {
		restore_name = p1
		current_value = p2
		restore_command = p3
		restore_file = restore_dir + "/" + restore_name
		if fi, err := os.Stat(restore_file); err == nil && fi.Mode().IsRegular() {
			restore_value = shCaptureEnv(`cat "$RESTORE_FILE"`, "RESTORE_FILE="+restore_file)
			if current_value != restore_value {
				// o original imprime ${command} (a variável global), não ${restore_command}
				fmt.Println("Executing: " + command)
				// eval "${restore_command}" — equivalente via sh -c
				shc(restore_command)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// increment_total
//
// Increment total count
//.

func increment_total() {
	total_count++
}

// ---------------------------------------------------------------------------
// increment_secure
//
// Increment secure count
//.

func increment_secure(p1 string) {
	if audit_mode != 2 {
		message = p1
		total_count++
		secure_count++
		if secure_count == 1 {
			printStyled(greenANSI, "[ SECURE ]", "  ", message+" ["+strconv.Itoa(secure_count)+" Pass]")
		} else {
			printStyled(greenANSI, "[ SECURE ]", "  ", message+" ["+strconv.Itoa(secure_count)+" Passes]")
		}
	}
}

// ---------------------------------------------------------------------------
// increment_insecure
//
// Increment insecure count
//.

func increment_insecure(p1 string) {
	if audit_mode != 2 {
		message = p1
		total_count++
		insecure_count++
		if insecure_count == 1 {
			verbose_message(message+" ["+strconv.Itoa(insecure_count)+" Warning]", "warn")
		} else {
			verbose_message(message+" ["+strconv.Itoa(insecure_count)+" Warnings]", "warn")
		}
	}
}

// ---------------------------------------------------------------------------
// check_aws
//
// Check AWS CLI etc is installed
//.

func check_aws() {
	aws_bin = shCaptureQuiet("command -v aws")
	if fi, err := os.Stat(aws_bin); err == nil && fi.Mode().IsRegular() {
		aws_creds = os.Getenv("HOME") + "/.aws/credentials"
		if _, err := os.Stat(aws_creds); err == nil {
			if os_name == "Darwin" {
				base64_d = "base64 -D"
			} else {
				base64_d = "base64 -d"
			}
		} else {
			fmt.Println("AWS credentials file does not exit")
			os.Exit(0)
		}
	} else {
		fmt.Println("AWS CLI is not installed")
		os.Exit(0)
	}
	if aws_region == "" {
		aws_region = execCapture("aws", "configure", "get", "region")
	}
}

// ---------------------------------------------------------------------------
// apply_latest_patches
//
// Code to apply patches
// Nothing done with this yet
//.

func apply_latest_patches() {
}

// ---------------------------------------------------------------------------
// secure_baseline
//
// Establish a Secure Baseline
// This uses the Solaris 10 svcadm baseline
// Don't really need this so haven't coded anything for it yet
//.

func secure_baseline() {
}

// ---------------------------------------------------------------------------
// Funções carregadas originalmente de ${app_dir}/core/*.sh (diretório ausente
// neste repositório). O bash sem set -e apenas reporta "command not found" em
// stderr e continua; o porte reproduz exatamente essa classe de comportamento.
// ---------------------------------------------------------------------------

func stub(name string) int {
	fmt.Fprintf(os.Stderr, "%s: command not found\n", name)
	return 127
}

// ---------------------------------------------------------------------------
// Saída formatada
// ---------------------------------------------------------------------------

// printStyled emite `${COR}[ LABEL ]${NC}   ${texto}` (echo -e do original).
func printStyled(cor, label, gaps, text string) {
	s := echoE(text) // o original usa echo -e na mensagem inteira
	if colorOn {
		fmt.Printf("%s%s%s%s%s\n", cor, label, ncANSI, gaps, s)
		return
	}
	fmt.Printf("%s%s%s\n", label, gaps, s)
}

// echoE interpreta os escapes do `echo -e` do bash.
func echoE(s string) string {
	var b strings.Builder
	isOct := func(c byte) bool { return c >= '0' && c <= '7' }
	isHex := func(c byte) bool {
		return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
	}
	hexVal := func(c byte) int {
		switch {
		case c >= '0' && c <= '9':
			return int(c - '0')
		case c >= 'a' && c <= 'f':
			return int(c-'a') + 10
		default:
			return int(c-'A') + 10
		}
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'e', 'E':
			b.WriteByte(0x1b)
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte('\v')
		case '\\':
			b.WriteByte('\\')
		case '0': // \0nnn — até 3 dígitos octais
			j, val, cnt := i+1, 0, 0
			for j < len(s) && cnt < 3 && isOct(s[j]) {
				val = val*8 + int(s[j]-'0')
				j++
				cnt++
			}
			b.WriteByte(byte(val))
			i = j - 1
		case 'x': // \xHH — até 2 dígitos hexadecimais
			j, val, cnt := i+1, 0, 0
			for j < len(s) && cnt < 2 && isHex(s[j]) {
				val = val*16 + hexVal(s[j])
				j++
				cnt++
			}
			if cnt == 0 {
				b.WriteByte('\\')
				b.WriteByte(s[i])
			} else {
				b.WriteByte(byte(val))
				i = j - 1
			}
		default:
			// escapes desconhecidos ficam literais, como no bash
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func prepararSaida() {
	if os.Getenv("NO_COLOR") != "" {
		return
	}
	if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		colorOn = true
	}
}

// ---------------------------------------------------------------------------
// Utilitários de execução (sem set -e: falhas não abortam)
// ---------------------------------------------------------------------------

func statusOf(err error) int {
	if err == nil {
		return 0
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode()
	}
	return 127
}

func runCmd(name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return statusOf(cmd.Run())
}

// shc roda um comando como o `sh -c "${command}"` do original.
func shc(command string) int {
	return runCmd("sh", "-c", command)
}

func sudoShc(command string) int {
	return runCmd("sudo", "sh", "-c", command)
}

// shCapture roda um pipeline e devolve a saída como `$( ... )` (sem os \n finais).
func shCapture(command string, env ...string) string {
	cmd := exec.Command("sh", "-c", command)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	cmd.Stderr = os.Stderr
	out, _ := cmd.Output()
	return strings.TrimRight(string(out), "\n")
}

// shCaptureEnv aceita variáveis de ambiente para os valores (evita injetar
// valores do usuário no texto do comando).
func shCaptureEnv(command string, env ...string) string {
	return shCapture(command, env...)
}

// shCaptureQuiet equivale a `$( comando 2> /dev/null )`.
func shCaptureQuiet(command string) string {
	cmd := exec.Command("sh", "-c", command)
	out, _ := cmd.Output()
	return strings.TrimRight(string(out), "\n")
}

func execCapture(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, _ := cmd.Output()
	return strings.TrimRight(string(out), "\n")
}

// script_path equivale a "$0": o launcher bash exporta LUNAR_SRC com o
// caminho real do script; sem ele (go run direto) usa o binário em execução.
func script_path() string {
	if s := os.Getenv("LUNAR_SRC"); s != "" {
		return s
	}
	return os.Args[0]
}

// bashErrText aproxima as mensagens do bash para falhas de redirecionamento.
func bashErrText(err error) string {
	if os.IsNotExist(err) {
		return "No such file or directory"
	}
	if os.IsPermission(err) {
		return "Permission denied"
	}
	return err.Error()
}

var stdinReader = bufio.NewReader(os.Stdin)

// readStdinLine equivale a `read -r reply` (sem tratar EOF: o chamador decide).
func readStdinLine() (string, error) {
	line, err := stdinReader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(line, "\n"), nil
}

// ---------------------------------------------------------------------------
// Handle command line arguments
// ---------------------------------------------------------------------------

func parseArgs() {
	// audit_mode=3 do_fs=3 audit_select=0 verbose_mode=0 force=0 do_select=0
	// do_aws=0 do_aws_rec=0 do_docker=0 print_funct=0 (defaults já declarados)
	audit_mode = 3
	do_fs = 3
	audit_select = 0
	verbose_mode = 0
	force = 0
	do_select = 0
	do_aws = 0
	do_aws_rec = 0
	do_docker = 0
	print_funct = 0

	argv := os.Args[1:]
	last := 0 // $? do bash no laço original
	i := 0
	arg2 := func() string {
		if i+1 < len(argv) {
			return argv[i+1]
		}
		return "" // $2 não existe → string vazia (o original não usa set -u)
	}
	shift := func(n int) {
		if n > len(argv)-i {
			// bash: "shift: n: shift count out of range"; $# não muda, o que no
			// original faz o laço se repetir (reproduzido aqui)
			fmt.Fprintf(os.Stderr, "lunar.sh: shift: %d: shift count out of range\n", n)
			last = 1
			return
		}
		i += n
		last = 0
	}

	// If given no command line arguments print usage information
	if strings.Join(argv, " ") == "" {
		last = stub("print_help")
		os.Exit(last) // `exit` sem argumento → status do print_help (127)
	}

	// Parse arguments
	for i < len(argv) {
		switch argv[i] {
		case "-1", "--list": // switch - List changes/backups
			list := arg2()
			if list == "" {
				last = stub("print_changes")
				last = stub("print_backups")
				shift(1)
			} else {
				switch list {
				case "changes":
					last = stub("print_changes")
				case "backups":
					last = stub("print_backups")
				case "tests":
					last = stub("print_tests")
				}
				shift(2)
			}
			os.Exit(last)
		case "-2", "--tests": // switch - Print tests
			tests := arg2()
			if tests == "" {
				last = stub("print_tests")
			} else if tests == "--verbose" {
				verbose_mode = 1
				last = stub("print_tests")
			} else {
				last = stub("print_tests")
				shift(2)
			}
			os.Exit(last)
		case "-3", "--printfunct": // switch - Print function
			print_funct = 1
			shift(1)
		case "-4", "--dryrun": // switch - Run in dryrun mode
			dryrun_mode = 1
			shift(1)
		case "-8", "--usesudo": // switch - Use sudo
			use_sudo = 1
			shift(1)
		case "-9", "--shellcheck": // switch - Run shellcheck against script
			last = stub("check_shellcheck")
			shift(1)
			os.Exit(last)
		case "-0", "--force": // switch - Force action
			force = 1
			shift(1)
		case "-a", "--audit": // switch - Run in audit mode (for Operating Systems - no changes made to system)
			audit_mode = 1
			do_fs = 0
			shift(1)
		case "-A", "--fullaudit": // switch - Run in audit mode and include filesystems (for Operating Systems - no changes made to system)
			audit_mode = 1
			do_fs = 1
			shift(1)
		case "-b", "--backups", "--listbackups": // switch - List backups
			last = stub("print_backups")
			shift(1)
			os.Exit(last)
		case "-B", "--basedir": // switch - Set base directory
			base_dir = arg2()
			shift(2)
		case "-c", "--codename", "--distro": // switch -  Distro/Code name (used with docker/multipass)
			test_distro = arg2()
			shift(2)
		case "-C", "--shell": // switch - Run docker-compose testing suite (drops to shell in order to do more testing)
			do_compose = 1
			do_shell = 1
			shift(1)
		case "-d", "--dockeraudit": // switch - Run in audit mode (for Docker - no changes made to system)
			audit_mode = 1
			do_docker = 1
			module_name = arg2()
			shift(2)
		case "-D", "--dockertests": // switch - List all Docker functions available to selective mode
			last = stub("print_tests")
			shift(1)
			os.Exit(last)
		case "-e", "--host": // switch - Run in audit mode on external host (for Operating Systems - no changes made to system)
			do_remote = 1
			ext_host = arg2()
			shift(2)
		case "-E", "--hash", "--passwordhash": // switch - Password hash
			password_hashing = arg2()
			shift(2)
		case "-f", "--action": // switch - Action (e.g delete - used with multipass)
			action = arg2()
			switch action {
			case "audit":
				audit_mode = 1
				do_fs = 0
			case "fullaudit":
				audit_mode = 1
				do_fs = 2
			case "lockdown":
				audit_mode = 0
				do_fs = 0
			case "undo", "restore":
				audit_mode = 2
			case "osinfo", "systeminfo":
				last = stub("check_os_release")
				os.Exit(last)
			}
			shift(2)
		case "-F", "--tempfile": // switch - Temporary file to use for operations
			temp_file = arg2()
			shift(2)
		case "-g", "--giturl": // switch - Git URL for code to copy to container
			git_url = arg2()
			shift(2)
		case "-G", "--wheelgroup": // switch - Set wheel group
			wheel_group = arg2()
			shift(2)
		case "-h", "--help": // switch - Display help
			last = stub("print_help")
			if verbose_mode == 1 {
				last = stub("print_usage")
			}
			shift(1)
			os.Exit(0)
		case "-H", "--usage": // switch - Display usage
			last = stub("print_usage")
			shift(1)
			os.Exit(last)
		case "-i", "--anacron": // switch - Enable/Disable anacron
			anacron_enable = arg2()
			shift(2)
		case "-I", "--type": // switch - Audit type
			audit_type = arg2()
			shift(2)
		case "-k", "--kubeaudit": // switch - Run in audit mode (for Kubernetes - no changes made to system)
			audit_mode = 1
			do_kubernetes = 1
			module_name = arg2()
			shift(2)
		case "-K", "--function", "--test": // switch - Do a specific function
			module_name = arg2()
			shift(2)
		case "-l", "--lockdown": // switch - Run in lockdown mode (for Operating Systems - changes made to system)
			audit_mode = 0
			do_fs = 0
			shift(1)
		case "-L", "--fulllockdown", "fulllock": // switch - Run in lockdown mode (for Operating Systems - changes made to system)
			audit_mode = 0
			do_fs = 1
			shift(1)
		case "-m", "--machine", "--vm": // switch - Set virtualisation type
			vm_type = arg2()
			switch vm_type {
			case "docker":
				do_compose = 1
				do_shell = 0
			case "multipass":
				do_multipass = 1
				do_shell = 0
			}
			shift(2)
		case "-M", "--workdir": // switch - Set work directory
			work_dir = arg2()
			shift(2)
		case "-n", "--ansible": // switch - Output ansible
			ansible_mode = 1
			shift(1)
		case "-N", "--nocat": // switch - Do output cat in score
			no_cat = 1
			shift(1)
		case "-o", "--os", "--osver": // switch - Set OS version
			test_os = arg2()
			shift(2)
		case "-O", "--osinfo", "--systeminfo": // switch - Print OS/System information
			last = stub("check_os_release")
			shift(1)
			os.Exit(last)
		case "-p", "--previous": // switch - Print previous audit information
			last = stub("print_previous")
			shift(1)
			os.Exit(last)
		case "-P", "--sshsandbox", "--sandbox": // switch - Enable/Disabe SSH sandbox
			ssh_sandbox = arg2()
			shift(2)
		case "-q", "--quiet", "--nostrict": // switch - Run in quiet mode
			// unset eu — a variável eu não existe no script: no-op fiel
			shift(1)
		case "-Q", "--debug": // switch - Run in debug mode
			do_debug = 1
			// `set -ux` (xtrace/nounset) do bash: não reproduzido no porte Go
			shift(1)
		case "-r", "--awsregion", "--region": // switch - Set AWS region
			aws_region = arg2()
			shift(2)
		case "-R", "--moduleinfo", "--testinfo": // switch - Print information about a module
			module = arg2()
			last = stub("print_audit_info")
			shift(2)
			os.Exit(last)
		case "-s", "--select", "--check": // switch - Run in selective mode (only run tests you want to)
			audit_mode = 1
			do_fs = 0
			do_select = 1
			module_name = arg2()
			shift(2)
		case "-S", "--unixtests", "--unix": // switch - List UNIX tests
			last = stub("print_tests")
			shift(1)
			os.Exit(last)
		case "-t", "--tag", "--name": // switch - Set docker tag
			test_tag = arg2()
			shift(2)
		case "-T", "--tempdir": // switch - Set temp directoru
			temp_dir = arg2()
			shift(2)
		case "-u", "--undo": // switch - Undo lockdown (for Operating Systems - changes made to system)
			audit_mode = 2
			restore_date = arg2()
			shift(2)
		case "-U", "--dofiles": // switch - Include filesystems
			do_fs = 1
			shift(1)
		case "-v", "--verbose": // switch - Run in verbose mode
			verbose_mode = 1
			shift(1)
		case "-V", "--version": // switch - Print version
			last = stub("print_version")
			shift(1)
			os.Exit(last)
		case "-w", "--awsaudit": // switch - Run in audit mode (for AWS - no changes made to system)
			audit_mode = 1
			do_aws = 1
			module_name = arg2()
			shift(2)
		case "-W", "--awstests", "--aws": // switch - List all AWS functions available to selective mode
			last = stub("print_tests")
			shift(1)
			os.Exit(last)
		case "-x", "--awsrec": // switch - Run in recommendations mode (for AWS - no changes made to system)
			audit_mode = 1
			do_aws_rec = 1
			module_name = arg2()
			shift(2)
		case "-X", "--strict": // switch - Run shellcheck against script
			// unset eu — no-op fiel (a variável eu não existe)
			shift(1)
			os.Exit(last)
		case "-z": // switch - Run specified audit function in lockdown mode
			audit_mode = 0
			do_fs = 0
			do_select = 1
			module_name = arg2()
			shift(2)
		case "-Z", "--changes", "--listchanges": // switch - List changes
			last = stub("print_changes")
			shift(1)
			os.Exit(last)
		default:
			stub("print_help") // print_help >&2
			os.Exit(1)
		}
	}
}

// ---------------------------------------------------------------------------
// Run in docker or multipass
// ---------------------------------------------------------------------------

func runDockerOrMultipass() int {
	if do_multipass == 1 {
		stub("get_ubuntu_codename")
		test_os = ubuntu_codename
		if test_tag == "none" {
			if test_os != "none" {
				test_tag = pkg_suffix + "-" + test_os
			}
		}
	}
	if test_os == "none" || test_tag == "none" {
		fmt.Println("OS name or version not given")
		os.Exit(0)
	}
	if do_compose == 1 {
		d_test := shCapture("command -v docker-compose ")
		if d_test == "" {
			verbose_message("Command docker-compose not found", "warn")
			os.Exit(0)
		}
		// cd "${app_dir}" || exit ; export OS_NAME=... ; export OS_VERSION=...
		if err := os.Chdir(app_dir); err != nil {
			os.Exit(1)
		}
		os.Setenv("OS_NAME", test_os)
		os.Setenv("OS_VERSION", test_tag)
		if do_shell == 0 {
			return runCmd("docker-compose", "run", "test-audit")
		}
		return runCmd("docker-compose", "run", "test-shell")
	}
	mpackage_test := shCapture(`command -v multipass | wc -l | sed "s/ //g"`)
	if mpackage_test != "1" {
		verbose_message("Command multipass not found", "warn")
		os.Exit(0)
	}
	vm_test := shCaptureEnv(`multipass list | awk '{print $1}' | grep -c "$TEST_TAG"`, "TEST_TAG="+test_tag)
	if vm_test == "1" {
		if action == "delete" {
			verbose_message("Multipass VM \""+test_tag+"\" with release \""+test_os+"\"", "delete")
			runCmd("multipass", "delete", test_tag)
			return runCmd("multipass", "purge")
		}
		if do_shell == 1 || action == "shell" {
			return runCmd("multipass", "shell", test_tag)
		}
		if action == "refresh" {
			runCmd("multipass", "exec", test_tag, "--", "bash", "-c", "rm -rf lunar")
			return runCmd("multipass", "exec", test_tag, "--", "bash", "-c", "git clone "+git_url)
		}
		mp_command := "sudo lunar/lunar.sh --action " + action
		if do_debug == 1 {
			mp_command = mp_command + " --debug"
		}
		if do_select == 1 {
			mp_command = mp_command + " --select " + module_name
		}
		return runCmd("multipass", "exec", test_tag, "--", "bash", "-c", mp_command)
	}
	if action == "delete" {
		// o original chama verbose_message sem o argumento de estilo
		verbose_message("Multipass VM \""+test_tag+"\" does not exist", "")
		os.Exit(0)
	}
	if action == "create" {
		verbose_message("Multipass VM \""+test_tag+"\" with release \""+test_os+"\"", "create")
		runCmd("multipass", "launch", test_os, "--name", test_tag)
		return runCmd("multipass", "exec", test_tag, "--", "bash", "-c", "git clone "+git_url)
	}
	// cai no `exit` final do bloco com o status do último comando (0 aqui)
	return 0
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	prepararSaida()

	// Defaults calculados em tempo de execução (mesma ordem do original)
	app_dir = execCapture("dirname", script_path())
	args = strings.Join(os.Args[1:], " ")
	date_suffix = execCapture("date", "+%d_%m_%Y_%H_%M_%S")
	temp_file = temp_dir + "/" + pkg_suffix + ".tmp"
	work_dir = base_dir + "/" + date_suffix
	my_id = execCapture("id", "-u")
	core_dir = app_dir + "/core"
	functions_dir = app_dir + "/functions"
	modules_dir = app_dir + "/modules"
	private_dir = app_dir + "/private"

	// Get our ID
	os_name = execCapture("uname")
	if os_name != "VMkernel" {
		if os_name == "SunOS" {
			id_check = shCapture("id | cut -c5")
		} else {
			id_check = execCapture("id", "-u")
		}
		arg_test = shCaptureEnv(`echo "$ARGS" | grep -cE "\-h|\-V|\-\-help|\-\-version|info"`, "ARGS="+args)
		if arg_test != "1" {
			if id_check != "0" {
				verbose_message(os.Args[0]+" may need root", "warn")
			}
		}
	}

	// Reset base dirs if not running as root
	if id_check != "0" {
		base_dir = os.Getenv("HOME") + "/." + pkg_suffix
		temp_dir = base_dir + "/tmp"
		temp_file = temp_dir + "/" + pkg_suffix + ".tmp"
		work_dir = base_dir + "/" + date_suffix
	}

	// Load core functions from core directory
	// (o original faz `. core/*.sh`: sourcing de shell no processo atual não é
	//  possível em Go; o diretório core/ não existe neste repositório, então o
	//  ramo é morto e as funções externas viram os stubs "command not found")
	if fi, err := os.Stat(core_dir); err == nil && fi.IsDir() {
		if verbose_mode == 1 {
			// no original estas mensagens nunca aparecem: o diretório é lido
			// antes do parser e verbose_mode ainda é 0 neste ponto
			fmt.Println()
			fmt.Println("Loading core functions")
			fmt.Println()
		}
	}

	// Install packages
	install_rsyslog = "no"

	// This is the company name that will go into the security message
	// Change it as required
	company_name = "Insert Company Name Here"

	// Get the path the script starts from
	start_path = execCapture("pwd")

	// Get the version of the script from the script itself
	script_version = shCaptureEnv(`grep '^# Version:' < "$LUNAR_SCRIPT" | awk '{print $3}'`, "LUNAR_SCRIPT="+script_path())

	// Handle command line arguments
	parseArgs()

	// Set Restore Directory if not set
	if audit_mode == 2 {
		restore_dir = base_dir + "/" + restore_date
	} else {
		if restore_dir == "" {
			restore_dir = work_dir
		}
	}

	// If running in dry run mode say so
	if dryrun_mode == 1 {
		verbose_message("Running in dryrun mode", "notice")
	}

	// If we are in lockdown mode do a check whether we are in force mode and whether to proceed
	if audit_mode == 0 {
		lockdown_warning()
	}

	// Get function name
	module_name = shCaptureEnv(`echo "$MODULE_NAME" | tr '[:upper:]' '[:lower:]' | sed "s/ /_/g"`, "MODULE_NAME="+module_name)

	// check arguments
	if do_audit == 1 {
		if audit_type == "" {
			audit_type = "local"
		}
	}

	// Run script remotely
	if do_remote == 1 {
		fmt.Printf("Copying %s to %s:/tmp\n", app_dir, ext_host)
		runCmd("scp", "-r", app_dir, ext_host+":/tmp")
		fmt.Printf("Executing lunar in audit mode (no changes will be made) on %s\n", ext_host)
		last := runCmd("ssh", ext_host, "sudo sh -c \""+temp_dir+"/lunar.sh -a\"")
		os.Exit(last)
	}

	stub("check_environment")

	// Run in docker or multipass
	if do_compose == 1 || do_multipass == 1 {
		os.Exit(runDockerOrMultipass())
	}

	if audit_mode != 3 {
		fmt.Println()
		if audit_mode == 2 {
			verbose_message("In Restore mode (changes will be made to system)", "run")
			verbose_message("Restore date "+restore_date, "set")
		}
		if audit_mode == 1 {
			verbose_message("In audit mode (no changes will be made to system)", "run")
		}
		if audit_mode == 0 {
			verbose_message("In lockdown mode (changes will be made to system)", "run")
		}
		if do_fs == 1 {
			verbose_message("Filesystem checks will be done", "info")
		}
		fmt.Println()
		if do_select == 1 {
			verbose_message("Selecting "+module_name, "audit")
			last := stub("funct_audit_select")
			os.Exit(last)
		}
		switch audit_type {
		case "Kubernetes":
			verbose_message("Kubernetes", "audit")
			last := stub("funct_audit_kubernetes")
			os.Exit(last)
		case "docker":
			verbose_message("Docker", "audit")
			last := stub("funct_audit_docker")
			os.Exit(last)
		case "awsrecommended":
			verbose_message("AWS - Recommended Tests", "audit")
			last := stub("funct_audit_aws_rec")
			os.Exit(last)
		case "aws":
			verbose_message("AWS", "audit")
			last := stub("funct_audit_aws")
			os.Exit(last)
		case "local", "system", "os":
			verbose_message("Operating System", "audit")
			last := stub("funct_audit_system")
			os.Exit(last)
		}
		// case sem correspondência: status 0 → `exit` do fim do bloco
		os.Exit(0)
	}
	// fim do script: `if` com condição falsa → status 0
	os.Exit(0)
}

// Variáveis de pacote espelhadas do bloco de instalação do original
// (não alteram o fluxo neste arquivo)

var (
	install_rsyslog = "no"
	company_name    = "Insert Company Name Here"
)

// --- fim do arquivo polyglot (lunar.go/lunar.sh) ---

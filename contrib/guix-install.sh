#!/bin/sh
# GNU Guix --- Functional package management for GNU
# Copyright © 2017 sharlatan <sharlatanus@gmail.com>
# Copyright © 2018 Ricardo Wurmus <rekado@elephly.net>
# Copyright © 2018 Efraim Flashner <efraim@flashner.co.il>
# Copyright © 2019–2020, 2022 Tobias Geerinckx-Rice <me@tobias.gr>
# Copyright © 2020 Morgan Smith <Morgan.J.Smith@outlook.com>
# Copyright © 2020 Simon Tournier <zimon.toutoune@gmail.com>
# Copyright © 2020 Daniel Brooks <db48x@db48x.net>
# Copyright © 2021 Jakub Kądziołka <kuba@kadziolka.net>
# Copyright © 2021 Chris Marusich <cmmarusich@gmail.com>
# Copyright © 2021, 2022, 2023 Maxim Cournoyer <maxim.cournoyer@gmail.com>
# Copyright © 2022 Prafulla Giri <prafulla.giri@protonmail.com>
# Copyright © 2023 Andrew Tropin <andrew@trop.in>
#
# This file is part of GNU Guix.
#
# GNU Guix is free software; you can redistribute it and/or modify it
# under the terms of the GNU General Public License as published by
# the Free Software Foundation; either version 3 of the License, or (at
# your option) any later version.
#
# GNU Guix is distributed in the hope that it will be useful, but
# WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with GNU Guix.  If not, see <http://www.gnu.org/licenses/>.

# We require Bash but for portability we'd rather not use /bin/bash or
# /usr/bin/env in the shebang, hence this hack.

# Environment variables
#
# GUIX_BINARY_FILE_NAME
#
# Can be used to override the automatic download mechanism and point
# to a local Guix binary archive filename like
# "/tmp/guix-binary-1.4.0rc2.armhf-linux.tar.xz"
#
# GUIX_ALLOW_OVERWRITE
#
# Instead of aborting to avoid overwriting a previous installations,
# allow copying over /var/guix or /gnu.  This can be useful when the
# installation required the user to extract Guix packs under /gnu to
# satisfy its dependencies.

if [ "x$BASH_VERSION" = "x" ]
then
    exec bash "$0" "$@"
fi

set -eo pipefail

[ "$UID" -eq 0 ] || { echo "This script must be run as root."; exit 1; }

REQUIRE=(
    "dirname"
    "readlink"
    "wget"
    "gpg"
    "grep"
    "which"
    "sed"
    "sort"
    "getent"
    "mktemp"
    "rm"
    "chmod"
    "uname"
    "groupadd"
    "useradd"
    "tail"
    "tr"
    "xz"
)

PAS=$'[ \033[32;1mPASS\033[0m ] '
ERR=$'[ \033[31;1mFAIL\033[0m ] '
WAR=$'[ \033[33;1mWARN\033[0m ] '
INF="[ INFO ] "

DEBUG=0
GNU_URL="https://ftp.gnu.org/gnu/guix/"
#GNU_URL="https://alpha.gnu.org/gnu/guix/"

# The following associative array holds set of GPG keys used to sign the
# releases, keyed by their corresponding Savannah user ID.
declare -A GPG_SIGNING_KEYS
GPG_SIGNING_KEYS[15145]=3CE464558A84FDC69DB40CFB090B11993D9AEBB5  # ludo
GPG_SIGNING_KEYS[127547]=27D586A4F8900854329FF09F1260E46482E63562 # maxim

# ------------------------------------------------------------------------------
#+UTILITIES

_err()
{ # All errors go to stderr.
    printf "[%s]: %s\n" "$(date +%s.%3N)" "$1"
}

_msg()
{ # Default message to stdout.
    printf "[%s]: %s\n" "$(date +%s.%3N)" "$1"
}

_debug()
{
    if [ "${DEBUG}" = '1' ]; then
        printf "[%s]: %s\n" "$(date +%s.%3N)" "$1"
    fi
}

die()
{
    _err "${ERR}$*"
    exit 1
}

# Return true if user answered yes, false otherwise.  The prompt is
# yes-biased, that is, when the user simply enter newline, it is equivalent to
# answering "yes".
# $1: The prompt question.
prompt_yes_no() {
    local -l yn
    read -rp "$1 [Y/n]" yn
    [[ ! $yn || $yn = y || $yn = yes ]] || return 1
}

chk_require()
{ # Check that every required command is available.
    declare -a warn
    local c

    _debug "--- [ ${FUNCNAME[0]} ] ---"

    for c in "$@"; do
        command -v "$c" &>/dev/null || warn+=("$c")
    done

    [ "${#warn}" -ne 0 ] && die "Missing commands: ${warn[*]}."

    _msg "${PAS}verification of required commands completed"
}

chk_gpg_keyring()
{ # Check whether the Guix release signing public key is present.
    _debug "--- [ ${FUNCNAME[0]} ] ---"
    local user_id
    local gpg_key_id
    local exit_flag

    for user_id in "${!GPG_SIGNING_KEYS[@]}"; do
        gpg_key_id=${GPG_SIGNING_KEYS[$user_id]}
        # Without --dry-run this command will create a ~/.gnupg owned by root on
        # systems where gpg has never been used, causing errors and confusion.
        if gpg --dry-run --list-keys "$gpg_key_id" >/dev/null 2>&1; then
            continue
        fi
        if prompt_yes_no "${INF}The following OpenPGP public key is \
required to verify the Guix binary signature: $gpg_key_id.
Would you like me to fetch it for you?"; then
            # Use a reasonable time-out here so users don't report silent
            # ‘freezes’ when Savannah goes out to lunch, as has happened.
            if wget "https://sv.gnu.org/people/viewgpg.php?user_id=$user_id" \
                    --timeout=30 --no-verbose -O- | gpg --import -; then
                continue
            fi
        fi
	# If we reach this point, the key is (still) missing.  Report further
	# missing keys, if any, but then abort the installation.
        _err "${ERR}Missing OpenPGP public key ($gpg_key_id).
Fetch it with this command:

  wget \"https://sv.gnu.org/people/viewgpg.php?user_id=$user_id\" -O - | \
sudo -i gpg --import -"
        exit_flag=yes
    done
    if [ "$exit_flag" = yes ]; then
        exit 1
    fi
}

chk_term()
{ # Check for ANSI terminal for color printing.
    if [ -t 2 ]; then
        if [ "${TERM+set}" = 'set' ]; then
            case "$TERM" in
                xterm*|rxvt*|urxvt*|linux*|vt*|eterm*|screen*)
                    ;;
                *)
                    ERR="[ FAIL ] "
                    PAS="[ PASS ] "
                    ;;
            esac
        fi
    fi
}

chk_init_sys()
{ # Return init system type name.
    if [[ $(/sbin/init --version 2>/dev/null) =~ upstart ]]; then
        _msg "${INF}init system is: upstart"
        INIT_SYS="upstart"
        return 0
    elif [[ $(systemctl 2>/dev/null) =~ -\.mount ]]; then
        _msg "${INF}init system is: systemd"
        INIT_SYS="systemd"
        return 0
    elif [[ -f /etc/init.d/cron && ! -h /etc/init.d/cron ]]; then
        _msg "${INF}init system is: sysv-init"
        INIT_SYS="sysv-init"
        return 0
    elif [[ $(openrc --version 2>/dev/null) =~ \(OpenRC\) ]]; then
        _msg "${INF}init system is: OpenRC"
        INIT_SYS="openrc"
        return 0
    else
        INIT_SYS="NA"
        _err "${ERR}Init system could not be detected."
    fi
}

chk_sys_arch()
{ # Check for operating system and architecture type.
    local os
    local arch

    os="$(uname -s)"
    arch="$(uname -m)"

    case "$arch" in
        i386 | i486 | i686 | i786 | x86)
            local arch=i686
            ;;
        x86_64 | x86-64 | x64 | amd64)
            local arch=x86_64
            ;;
        aarch64)
            local arch=aarch64
            ;;
        armv7l)
            local arch=armhf
            ;;
        ppc64le | powerpc64le)
            local arch=powerpc64le
            ;;
        *)
            die "Unsupported CPU type: ${arch}"
    esac

    case "$os" in
        Linux | linux)
            local os=linux
            ;;
        *)
            die "Your operation system (${os}) is not supported."
    esac

    ARCH_OS="${arch}-${os}"
}

chk_sys_nscd()
{ # Check if nscd is up and suggest to start it or install it
    if [ "$(type -P pidof)" ]; then
        if [ ! "$(pidof nscd)" ]; then
            _msg "${WAR}We recommend installing and/or starting your distribution 'nscd' service"
            _msg "${WAR}Please read 'info guix \"Application Setup\"' about \"Name Service Switch\""
        fi
    else
        _msg "${INF}We cannot determine if your distribution 'nscd' service is running"
        _msg "${INF}Please read 'info guix \"Application Setup\"' about \"Name Service Switch\""
    fi
}

# Configure substitute discovery according to user's preferences.
# $1 is the installed service file to edit.
configure_substitute_discovery() {
    if grep -q -- '--discover=no' "$1" && \
            prompt_yes_no "Would you like the Guix daemon to automatically \
discover substitute servers on the local network?"; then
        sed -i 's/--discover=no/--discover=yes/' "$1"
    fi
}

# ------------------------------------------------------------------------------
#+MAIN

guix_get_bin_list()
{ # Scan GNU archive and save list of binaries
    local gnu_url="$1"
    local -a bin_ver_ls
    local latest_ver
    local default_ver

    _debug "--- [ ${FUNCNAME[0]} ] ---"

    # Filter only version and architecture
    bin_ver_ls=("$(wget "$gnu_url" --no-verbose -O- \
        | sed -n -e 's/.*guix-binary-\([0-9.]*[a-z0-9]*\)\..*.tar.xz.*/\1/p' \
        | sort -Vu)")

    latest_ver="$(echo "${bin_ver_ls[0]}" \
                       | grep -oE "([0-9]{1,2}\.){2}[0-9]{1,2}[a-z0-9]*" \
                       | tail -n1)"

    default_ver="guix-binary-${latest_ver}.${ARCH_OS}"

    if [[ "${#bin_ver_ls}" -ne "0" ]]; then
        _msg "${PAS}Release for your system: ${default_ver}"
    else
        die "Could not obtain list of Guix releases."
    fi

    # Use default to download according to the list and local ARCH_OS.
    BIN_VER="${default_ver}"
}

guix_get_bin()
{ # Download and verify binary package.
    local url="$1"
    local bin_ver="$2"
    local dl_path="$3"
    local wget_args=()

    _debug "--- [ ${FUNCNAME[0]} ] ---"

    _msg "${INF}Downloading Guix release archive"

    wget --help | grep -q '\--show-progress' \
        && wget_args=("--no-verbose" "--show-progress")

    if wget "${wget_args[@]}" -P "$dl_path" \
            "${url}/${bin_ver}.tar.xz" "${url}/${bin_ver}.tar.xz.sig"; then
        _msg "${PAS}download completed."
    else
        die "could not download ${url}/${bin_ver}.tar.xz."
    fi

    pushd "${dl_path}" >/dev/null
    if gpg --verify "${bin_ver}.tar.xz.sig" >/dev/null 2>&1; then
        _msg "${PAS}Signature is valid."
        popd >/dev/null
    else
        die "could not verify the signature."
    fi
}

sys_create_store()
{ # Unpack and install /gnu/store and /var/guix
    local pkg="$1"
    local tmp_path="$2"

    _debug "--- [ ${FUNCNAME[0]} ] ---"

    if [[ -e /var/guix && -e /gnu ]]; then
        if [ -n "$GUIX_ALLOW_OVERWRITE" ]; then
            _msg "${WAR}Overwriting existing installation!"
        else
            die "A previous Guix installation was found.  Refusing to overwrite."
        fi
    fi

    cd "$tmp_path"
    _msg "${INF}Installing /var/guix and /gnu..."
    tar --extract --file "$pkg" -C /

    _msg "${INF}Linking the root user's profile"
    mkdir -p ~root/.config/guix
    ln -sf /var/guix/profiles/per-user/root/current-guix \
       ~root/.config/guix/current

    GUIX_PROFILE=~root/.config/guix/current
    # shellcheck disable=SC1090
    source "${GUIX_PROFILE}/etc/profile"
    _msg "${PAS}activated root profile at ${GUIX_PROFILE}"
}

sys_create_build_user()
{ # Create the group and user accounts for build users.

    _debug "--- [ ${FUNCNAME[0]} ] ---"

    if getent group guixbuild > /dev/null; then
        _msg "${INF}group guixbuild exists"
    else
        groupadd --system guixbuild
        _msg "${PAS}group <guixbuild> created"
    fi

    if getent group kvm > /dev/null; then
        _msg "${INF}group kvm exists and build users will be added to it"
        local KVMGROUP=,kvm
    fi

    for i in $(seq -w 1 10); do
        if id "guixbuilder${i}" &>/dev/null; then
            _msg "${INF}user is already in the system, reset"
            usermod -g guixbuild -G guixbuild${KVMGROUP}     \
                    -d /var/empty -s "$(which nologin)" \
                    -c "Guix build user $i"             \
                    "guixbuilder${i}";
        else
            useradd -g guixbuild -G guixbuild${KVMGROUP}     \
                    -d /var/empty -s "$(which nologin)" \
                    -c "Guix build user $i" --system    \
                    "guixbuilder${i}";
            _msg "${PAS}user added <guixbuilder${i}>"
        fi
    done
}

sys_enable_guix_daemon()
{ # Run the daemon, and set it to automatically start on boot.

    local info_path
    local local_bin
    local var_guix

    _debug "--- [ ${FUNCNAME[0]} ] ---"

    info_path="/usr/local/share/info"
    local_bin="/usr/local/bin"
    var_guix="/var/guix/profiles/per-user/root/current-guix"

    case "$INIT_SYS" in
        upstart)
            { initctl reload-configuration;
              cp ~root/.config/guix/current/lib/upstart/system/guix-daemon.conf \
                 /etc/init/ &&
                  configure_substitute_discovery /etc/init/guix-daemon.conf &&
                  start guix-daemon; } &&
                _msg "${PAS}enabled Guix daemon via upstart"
            ;;
        systemd)
            { # systemd .mount units must be named after the target directory.
              # Here we assume a hard-coded name of /gnu/store.
              # XXX Work around <https://issues.guix.gnu.org/41356> until next release.
              if [ -f ~root/.config/guix/current/lib/systemd/system/gnu-store.mount ]; then
                  cp ~root/.config/guix/current/lib/systemd/system/gnu-store.mount \
                     /etc/systemd/system/;
                  chmod 664 /etc/systemd/system/gnu-store.mount;
                  systemctl daemon-reload &&
                      systemctl enable gnu-store.mount;
              fi

              cp ~root/.config/guix/current/lib/systemd/system/guix-daemon.service \
                 /etc/systemd/system/;
              chmod 664 /etc/systemd/system/guix-daemon.service;

              # Work around <https://bugs.gnu.org/36074>, present in 1.0.1.
              sed -i /etc/systemd/system/guix-daemon.service \
                  -e "s/GUIX_LOCPATH='/'GUIX_LOCPATH=/";

              # Work around <https://bugs.gnu.org/35671>, present in 1.0.1.
              if ! grep en_US /etc/systemd/system/guix-daemon.service >/dev/null;
              then sed -i /etc/systemd/system/guix-daemon.service \
                       -e 's/^Environment=\(.*\)$/Environment=\1 LC_ALL=en_US.UTF-8';
              fi;

              configure_substitute_discovery \
                  /etc/systemd/system/guix-daemon.service

              systemctl daemon-reload &&
                  systemctl enable guix-daemon &&
                  systemctl start  guix-daemon; } &&
                _msg "${PAS}enabled Guix daemon via systemd"
            ;;
        sysv-init)
            { mkdir -p /etc/init.d;
              cp ~root/.config/guix/current/etc/init.d/guix-daemon \
                 /etc/init.d/guix-daemon;
              chmod 775 /etc/init.d/guix-daemon;

              configure_substitute_discovery /etc/init.d/guix-daemon

              update-rc.d guix-daemon defaults &&
                  update-rc.d guix-daemon enable &&
                  service guix-daemon start; } &&
                _msg "${PAS}enabled Guix daemon via sysv"
            ;;
        openrc)
            { mkdir -p /etc/init.d;
              cp ~root/.config/guix/current/etc/openrc/guix-daemon \
                 /etc/init.d/guix-daemon;
              chmod 775 /etc/init.d/guix-daemon;

              configure_substitute_discovery /etc/init.d/guix-daemon

              rc-update add guix-daemon default &&
                  rc-service guix-daemon start; } &&
                _msg "${PAS}enabled Guix daemon via OpenRC"
            ;;
        NA|*)
            _msg "${ERR}unsupported init system; run the daemon manually:"
            echo "  ~root/.config/guix/current/bin/guix-daemon --build-users-group=guixbuild"
            ;;
    esac

    _msg "${INF}making the guix command available to other users"

    [ -e "$local_bin" ] || mkdir -p "$local_bin"
    ln -sf "${var_guix}/bin/guix"  "$local_bin"

    [ -e "$info_path" ] || mkdir -p "$info_path"
    for i in "${var_guix}"/share/info/*; do
        ln -sf "$i" "$info_path"
    done
}

sys_authorize_build_farms()
{ # authorize the public key(s) of the build farm(s)
    local hosts=(
	ci.guix.gnu.org
	bordeaux.guix.gnu.org
    )

    if prompt_yes_no "Permit downloading pre-built package binaries from the \
project's build farms?"; then
	for host in "${hosts[@]}"; do
	    local key=~root/.config/guix/current/share/guix/$host.pub
	    [ -f "$key" ] \
		&& guix archive --authorize < "$key" \
		&& _msg "${PAS}Authorized public key for $host"
	done
    else
        _msg "${INF}Skipped authorizing build farm public keys"
    fi
}

sys_create_init_profile()
{ # Define for better desktop integration
  # This will not take effect until the next shell or desktop session!
    [ -d "/etc/profile.d" ] || mkdir /etc/profile.d # Just in case
    cat <<"EOF" > /etc/profile.d/zzz-guix.sh
# Explicitly initialize XDG base directory variables to ease compatibility
# with Guix System: see <https://issues.guix.gnu.org/56050#3>.
export XDG_DATA_HOME="${XDG_DATA_HOME:-$HOME/.local/share}"
export XDG_CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
export XDG_STATE_HOME="${XDG_STATE_HOME:-$HOME/.local/state}"
export XDG_DATA_DIRS="${XDG_DATA_DIRS:-/usr/local/share/:/usr/share/}"
export XDG_CONFIG_DIRS="${XDG_CONFIG_DIRS:-/etc/xdg}"
export XDG_CACHE_HOME="${XDG_CACHE_HOME:-$HOME/.cache}"
# no default for XDG_RUNTIME_DIR (depends on foreign distro for semantics)

# _GUIX_PROFILE: `guix pull` profile
_GUIX_PROFILE="$HOME/.config/guix/current"
export PATH="$_GUIX_PROFILE/bin${PATH:+:}$PATH"
# Export INFOPATH so that the updated info pages can be found
# and read by both /usr/bin/info and/or $GUIX_PROFILE/bin/info
# When INFOPATH is unset, add a trailing colon so that Emacs
# searches 'Info-default-directory-list'.
export INFOPATH="$_GUIX_PROFILE/share/info:$INFOPATH"

# GUIX_PROFILE: User's default profile and home profile
GUIX_PROFILE="$HOME/.guix-profile"
[ -f "$GUIX_PROFILE/etc/profile" ] && . "$GUIX_PROFILE/etc/profile"
[ -L "$GUIX_PROFILE" ] || \
GUIX_LOCPATH="$GUIX_PROFILE/lib/locale:${GUIX_LOCPATH:+:}$GUIX_LOCPATH"

GUIX_PROFILE="$HOME/.guix-home/profile"
[ -f "$GUIX_PROFILE/etc/profile" ] && . "$GUIX_PROFILE/etc/profile"
[ -L "$GUIX_PROFILE" ] || \
GUIX_LOCPATH="$GUIX_PROFILE/lib/locale:${GUIX_LOCPATH:+:}$GUIX_LOCPATH"

export GUIX_LOCPATH

EOF
}

sys_create_shell_completion()
{ # Symlink supported shell completions system-wide

    var_guix=/var/guix/profiles/per-user/root/current-guix
    bash_completion=/etc/bash_completion.d
    zsh_completion=/usr/share/zsh/site-functions
    fish_completion=/usr/share/fish/vendor_completions.d

    { # Just in case
        for dir_shell in $bash_completion $zsh_completion $fish_completion; do
            [ -d "$dir_shell" ] || mkdir -p $dir_shell
        done;

        ln -sf ${var_guix}/etc/bash_completion.d/* "$bash_completion";
        ln -sf ${var_guix}/share/zsh/site-functions/* "$zsh_completion";
        ln -sf ${var_guix}/share/fish/vendor_completions.d/* "$fish_completion"; } &&
        _msg "${PAS}installed shell completion"
}

sys_customize_bashrc()
{
    prompt_yes_no "Customize users Bash shell prompt for Guix?" || return
    for bashrc in /home/*/.bashrc /root/.bashrc; do
        test -f "$bashrc" || continue
        grep -Fq '$GUIX_ENVIRONMENT' "$bashrc" && continue
        cp "${bashrc}" "${bashrc}.bak"
        echo '
# Automatically added by the Guix install script.
if [ -n "$GUIX_ENVIRONMENT" ]; then
    if [[ $PS1 =~ (.*)"\\$" ]]; then
        PS1="${BASH_REMATCH[1]} [env]\\\$ "
    fi
fi
' >> "$bashrc"
    done
    _msg "${PAS}Bash shell prompt successfully customized for Guix"
}

welcome()
{
    local char
    cat<<"EOF"
    ░░░                                     ░░░
    ░░▒▒░░░░░░░░░               ░░░░░░░░░▒▒░░
     ░░▒▒▒▒▒░░░░░░░           ░░░░░░░▒▒▒▒▒░
         ░▒▒▒░░▒▒▒▒▒         ░░░░░░░▒▒░
               ░▒▒▒▒░       ░░░░░░
                ▒▒▒▒▒      ░░░░░░
                 ▒▒▒▒▒     ░░░░░
                 ░▒▒▒▒▒   ░░░░░
                  ▒▒▒▒▒   ░░░░░
                   ▒▒▒▒▒ ░░░░░
                   ░▒▒▒▒▒░░░░░
                    ▒▒▒▒▒▒░░░
                     ▒▒▒▒▒▒░
     _____ _   _ _    _    _____       _
    / ____| \ | | |  | |  / ____|     (_)
   | |  __|  \| | |  | | | |  __ _   _ ___  __
   | | |_ | . ' | |  | | | | |_ | | | | \ \/ /
   | |__| | |\  | |__| | | |__| | |_| | |>  <
    \_____|_| \_|\____/   \_____|\__,_|_/_/\_\

This script installs GNU Guix on your system

https://www.gnu.org/software/guix/
EOF
    # Don't use ‘read -p’ here!  It won't display when run non-interactively.
    echo -n "Press return to continue..."$'\r'
    if ! read -r char; then
	echo
	die "Can't read standard input.  Hint: don't pipe scripts into a shell."
    fi
    if [ "$char" ]; then
	echo
	echo "...that ($char) was not a return!"
	_msg "${WAR}Use newlines to automate installation, e.g.: yes '' | ${0##*/}"
	_msg "${WAR}Any other method is unsupported and likely to break in future."
    fi
}

main()
{
    local tmp_path
    welcome

    _msg "Starting installation ($(date))"

    chk_term
    chk_require "${REQUIRE[@]}"
    chk_gpg_keyring
    chk_init_sys
    chk_sys_arch
    chk_sys_nscd

    _msg "${INF}system is ${ARCH_OS}"

    umask 0022
    tmp_path="$(mktemp -t -d guix.XXXXXX)"

    if [ -z "${GUIX_BINARY_FILE_NAME}" ]; then
        guix_get_bin_list "${GNU_URL}"
        guix_get_bin "${GNU_URL}" "${BIN_VER}" "$tmp_path"
        GUIX_BINARY_FILE_NAME=${BIN_VER}.tar.xz
    else
        if ! [[ $GUIX_BINARY_FILE_NAME =~ $ARCH_OS ]]; then
            _err "$ARCH_OS not in ${GUIX_BINARY_FILE_NAME}; aborting"
        fi
        _msg "${INF}Using manually provided binary ${GUIX_BINARY_FILE_NAME}"
        GUIX_BINARY_FILE_NAME=$(realpath "$GUIX_BINARY_FILE_NAME")
    fi

    sys_create_store "${GUIX_BINARY_FILE_NAME}" "${tmp_path}"
    sys_create_build_user
    sys_enable_guix_daemon
    sys_authorize_build_farms
    sys_create_init_profile
    sys_create_shell_completion
    sys_customize_bashrc

    _msg "${INF}cleaning up ${tmp_path}"
    rm -r "${tmp_path}"

    _msg "${PAS}Guix has successfully been installed!"
    _msg "${INF}Run 'info guix' to read the manual."

    # Required to source /etc/profile in desktop environments.
    _msg "${INF}Please log out and back in to complete the installation."
 }

main "$@"

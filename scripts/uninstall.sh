#!/bin/sh
# Standalone recovery path: does not execute the installed binary.
set -eu
umask 077
fail() { printf '%s\n' "tofa: $*" >&2; exit 1; }
purge=no
case "${1:-}" in --purge) purge=yes;shift;; '') ;; *) fail 'usage: uninstall.sh [--purge]';; esac
[ "$#" -eq 0 ] || fail 'unexpected argument'
: "${HOME:?HOME is required}"
root=${TOFA_INSTALL_DIR:-"$HOME/.local/share/tofa"}
config=${XDG_CONFIG_HOME:-"$HOME/.config"}/tofa
case "$root:$config" in *'
'*) fail 'newlines in paths are unsupported';; esac
case "$root" in /*) ;; *) fail 'install directory must be absolute';; esac
case "$config" in /*) ;; *) fail 'configuration directory must be absolute';; esac
[ ! -L "$root" ] && [ ! -L "$config" ] || fail 'refusing symlink directory'
if [ "$purge" = yes ] && [ -d "$config" ];then
 # Retain references on any native-store failure so cleanup can be retried.
 [ ! -e "$config/.auth-lock" ] || fail 'authentication operation active (or stale .auth-lock); cleanup stopped'
 [ ! -L "$config/keyring-refs" ] || fail 'refusing symlink recovery directory'
 if [ -d "$config/keyring-refs" ];then
  for marker in "$config"/keyring-refs/*;do
   [ -e "$marker" ] || continue
   [ -f "$marker" ] && [ ! -L "$marker" ] || fail 'unexpected recovery entry'
   ref=${marker##*/}
   case "$ref" in *[!a-f0-9]*|'') fail 'invalid credential reference';; esac
   [ "${#ref}" -eq 32 ] || fail 'invalid credential reference'
   case "$(uname -s)" in
    Darwin)
     # security returns 44 when the item is already absent.
     code=0
     /usr/bin/security delete-generic-password -s io.nebius.tofa.prototype -a "$ref" >/dev/null 2>&1 || code=$?
     [ "$code" -eq 0 ] || [ "$code" -eq 44 ] || fail 'keychain cleanup failed; unlock it and rerun --purge; references preserved' ;;
    Linux)
     command -v secret-tool >/dev/null 2>&1 || fail 'keyring cleanup needs secret-tool (libsecret tools); install it and rerun --purge; references preserved'
     secret-tool clear service io.nebius.tofa.prototype username "$ref" || fail 'Secret Service cleanup failed; start/unlock the service and rerun --purge' ;;
    *) fail 'use uninstall.ps1 on Windows' ;;
   esac
   rm "$marker"
  done
 fi
 # Only known tofa data files are removed, never the whole config directory.
 rm -f "$config/config.yml" "$config/credentials.yml"
 rmdir "$config/keyring-refs" 2>/dev/null || :
 rmdir "$config" 2>/dev/null || :
fi
if [ -e "$root/.tofa-install" ];then
 [ ! -L "$root/.tofa-install" ] && [ "$(cat "$root/.tofa-install")" = tofa-install-v1 ] || fail 'unknown install ownership'
 [ ! -L "$root/bin" ] || fail 'refusing symlink bin directory'
 [ ! -L "$root/.path-files" ] || fail 'refusing symlink PATH metadata'
 if [ -f "$root/.path-files" ];then
  while IFS= read -r rc;do
   [ -f "$rc" ] || continue
   [ ! -L "$rc" ] || fail 'startup file is now a symlink; PATH cleanup stopped'
   temp=$(mktemp "${rc}.tofa.XXXXXX")
   if ! awk '/^# >>> tofa >>>$/{if(skip) exit 1;skip=1;next} /^# <<< tofa <<<$/{if(!skip) exit 1;skip=0;next} !skip{print} END{if(skip) exit 1}' "$rc" > "$temp";then
    rm "$temp";fail 'damaged tofa PATH markers; startup file retained—repair the marked block and retry'
   fi
   cat "$temp" > "$rc";rm "$temp"
  done < "$root/.path-files"
 fi
 rm -f "$root/bin/tofa" "$root/.tofa-install" "$root/.path-files"
 rmdir "$root/bin" 2>/dev/null || :
 rmdir "$root" 2>/dev/null || :
elif [ -e "$root/bin/tofa" ];then fail 'binary has no tofa ownership manifest; refusing removal'
fi
if [ "$purge" = yes ];then printf '%s\n' 'Removed tofa and its saved configuration/credentials. Unrelated files retained.'
else printf '%s\n' 'Removed tofa. Saved preferences and credentials retained; use --purge to remove them.';fi
printf '%s\n' 'PATH changes apply to future shells. Existing terminals may retain the old entry.'

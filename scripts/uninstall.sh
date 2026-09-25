#!/bin/sh
# Standalone recovery path. Desktop integration uses a verified bridge when
# the main installed executable has already been removed.
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
[ ! -L "$config/desktop-bridge-v1" ] && [ ! -L "$config/desktop-bridge-v2" ] || fail 'refusing symlink desktop integration'
if [ "$(uname -s)" = Darwin ] && [ "${TOFA_DESKTOP_LIFECYCLE_PARENT:-}" != "$PPID" ] && { [ -e "$config/desktop-bridge-v1" ] || [ -e "$config/desktop-bridge-v2" ]; }; then
 helper=
 # Prefer a retained coordinator even if the main installed binary is broken.
 for candidate in "$config"/desktop-bridge-v2/*/tofa-desktop-engine "$config"/desktop-bridge-v1/*/tofa-desktop-engine; do
  if [ -f "$candidate" ] && [ ! -L "$candidate" ] && [ -f "${candidate%/*}/owner.json" ] && grep -q '"LifecycleVersion":1[,}]' "${candidate%/*}/owner.json"; then helper=$candidate; break; fi
 done
 if [ -n "$helper" ]; then
  bridge_dir=${helper%/*}
  [ ! -L "$bridge_dir" ] && [ ! -L "${bridge_dir%/*}" ] && [ -f "$bridge_dir/owner.json" ] && [ ! -L "$bridge_dir/owner.json" ] || fail 'refusing unowned desktop recovery helper'
  # Check before executing: a user-edited executable cannot validate itself.
  expected=$(sed -n 's/.*"SHA256":"\([0-9a-f]*\)".*/\1/p' "$bridge_dir/owner.json")
  pending=$(sed -n 's/.*"PendingSHA256":"\([0-9a-f]*\)".*/\1/p' "$bridge_dir/owner.json")
  actual=$(shasum -a 256 "$helper" | awk '{print $1}')
  { [ "${#expected}" -eq 64 ] && [ "$actual" = "$expected" ]; } || { [ "${#pending}" -eq 64 ] && [ "$actual" = "$pending" ]; } || fail 'desktop recovery helper differs from its ownership record; restore matching artifacts or reinstall a compatible release'
  set -- --tofa-installed-lifecycle uninstall
 elif [ -f "$root/bin/tofa" ] && [ ! -L "$root/bin" ] && [ ! -L "$root/bin/tofa" ]; then
  [ -f "$root/.tofa-install" ] && [ ! -L "$root/.tofa-install" ] && [ "$(cat "$root/.tofa-install")" = tofa-install-v1 ] || fail 'unknown install ownership; desktop cleanup stopped'
  helper=$root/bin/tofa
  set -- desktop-lifecycle uninstall
 else
  fail 'desktop recovery helper is missing or incompatible; reinstall a compatible tofa release before uninstalling; shared state retained'
 fi
 if [ "$purge" = yes ]; then set -- "$@" --purge; fi
 exec "$helper" "$@"
fi
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
 rm -f "$root/bin/tofa" "$root/.path-files"
 rmdir "$root/bin" 2>/dev/null || :
 remaining=$(ls -A "$root")
 if [ "$purge" = no ] && [ "$remaining" != .tofa-install ];then
  # Unrelated files keep the directory alive. Retain proof of ownership so a
  # later installer can reuse it without accepting arbitrary unowned directories.
  printf '%s\n' 'Retained install ownership marker beside unrelated files so tofa can be reinstalled.'
 else
  rm -f "$root/.tofa-install"
 fi
 rmdir "$root" 2>/dev/null || :
elif [ -e "$root/bin/tofa" ];then fail 'binary has no tofa ownership manifest; refusing removal'
fi
if [ "$purge" = yes ];then printf '%s\n' 'Removed tofa and its saved configuration/credentials. Unrelated files retained.'
else printf '%s\n' 'Removed tofa. Saved preferences and credentials retained; use --purge to remove them.';fi
printf '%s\n' 'PATH changes apply to future shells. Existing terminals may retain the old entry.'

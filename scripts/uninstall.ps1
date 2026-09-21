param([switch]$Purge, [int]$WaitPid=0)
$ErrorActionPreference='Stop'
if ($WaitPid) {Wait-Process -Id $WaitPid -ErrorAction SilentlyContinue}
$Root=Join-Path $env:LOCALAPPDATA 'tofa\install'
if ($env:TOFA_INSTALL_DIR) {$Root=$env:TOFA_INSTALL_DIR}
$Config=Join-Path $env:LOCALAPPDATA 'tofa'
function Assert-NotLink([string]$Path) {
 if ((Test-Path -LiteralPath $Path) -and ((Get-Item -Force -LiteralPath $Path).Attributes -band [IO.FileAttributes]::ReparsePoint)) {throw "Refusing reparse point: $Path"}
}
Assert-NotLink $Root;Assert-NotLink $Config
if($Purge -and (Test-Path -LiteralPath $Config)) {
 if(Test-Path -LiteralPath (Join-Path $Config '.auth-lock')) {throw 'Authentication operation active or stale .auth-lock; cleanup stopped'}
 $Refs=Join-Path $Config 'keyring-refs';Assert-NotLink $Refs
 if(Test-Path -LiteralPath $Refs) {
  # Delete only tofa's recorded generic credentials. No key material is read.
  Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class TofaCredentials {
 [DllImport("advapi32.dll", EntryPoint="CredDeleteW", CharSet=CharSet.Unicode, SetLastError=true)]
 public static extern bool Delete(string target, uint type, uint flags);
}
'@
  foreach($File in Get-ChildItem -Force -LiteralPath $Refs) {
   Assert-NotLink $File.FullName
   if($File.PSIsContainer -or $File.Name -notmatch '^[a-f0-9]{32}$') {throw 'Unexpected credential reference'}
   $Deleted=[TofaCredentials]::Delete(('io.nebius.tofa.prototype:'+$File.Name),1,0)
   if(!$Deleted -and [Runtime.InteropServices.Marshal]::GetLastWin32Error() -ne 1168) {throw 'Credential Manager cleanup failed; references retained; rerun -Purge'}
   Remove-Item -LiteralPath $File.FullName
  }
  Remove-Item -LiteralPath $Refs
 }
 foreach($Name in @('config.yml','credentials.yml')) {
  $Path=Join-Path $Config $Name;Assert-NotLink $Path
  if(Test-Path -LiteralPath $Path){Remove-Item -LiteralPath $Path}
 }
}
$Manifest=Join-Path $Root '.tofa-install'
if(Test-Path -LiteralPath $Manifest){
 Assert-NotLink $Manifest
 if((Get-Content -Raw -LiteralPath $Manifest).Trim() -ne 'tofa-install-v1'){throw 'Unknown install ownership'}
 $Bin=Join-Path $Root 'bin';Assert-NotLink $Bin
 $PathMarker=Join-Path $Root '.path-owned';Assert-NotLink $PathMarker
 if(Test-Path -LiteralPath $PathMarker){
  if((Get-Content -Raw -LiteralPath $PathMarker).Trim() -ne $Bin){throw 'Invalid PATH ownership marker'}
  $Entries=@([Environment]::GetEnvironmentVariable('Path','User') -split ';' | Where-Object {$_ -and $_ -ne $Bin})
  [Environment]::SetEnvironmentVariable('Path',($Entries -join ';'),'User')
  Remove-Item -LiteralPath $PathMarker
 }
 $Exe=Join-Path $Bin 'tofa.exe';Assert-NotLink $Exe
 if(Test-Path -LiteralPath $Exe){Remove-Item -LiteralPath $Exe}
 if((Test-Path -LiteralPath $Bin) -and !(Get-ChildItem -Force -LiteralPath $Bin)){Remove-Item -LiteralPath $Bin}
 # Retain ownership for reinstall when unrelated files keep the directory alive.
 # Explicit purge relinquishes ownership even if those files remain.
 if($Purge -or !(Get-ChildItem -Force -LiteralPath $Root | Where-Object {$_.Name -ne '.tofa-install'})) {
  Remove-Item -LiteralPath $Manifest
 }
 if(!(Get-ChildItem -Force -LiteralPath $Root)){Remove-Item -LiteralPath $Root}
}elseif(Test-Path -LiteralPath (Join-Path $Root 'bin\tofa.exe')){throw 'Binary has no tofa ownership manifest; refusing removal'}
if($Purge){Write-Host 'Removed tofa and saved credentials/preferences. Unrelated files retained.'}
else{Write-Host 'Removed tofa; saved credentials/preferences retained. Use -Purge to remove them.'}
Write-Host 'Existing terminals may retain the old PATH entry.'

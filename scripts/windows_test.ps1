# Offline lifecycle test. Uses a temporary LOCALAPPDATA and no PATH/keyring changes.
param([switch]$InstallerOnly)
$ErrorActionPreference='Stop'
$Scripts=$PSScriptRoot
$Temp=Join-Path ([IO.Path]::GetTempPath()) ('tofa test '+[Guid]::NewGuid().ToString('N'))
$OriginalLocal=$env:LOCALAPPDATA
$OriginalBase=$env:TOFA_RELEASE_BASE_URL
$OriginalRoot=$env:TOFA_INSTALL_DIR
function Assert($Condition,[string]$Message){if(!$Condition){throw $Message}}
New-Item -ItemType Directory -Path $Temp | Out-Null
$global:TofaFixtureBytes=[Text.Encoding]::UTF8.GetBytes('fixture executable')
$global:TofaFixtureCorrupt=$false
$global:TofaFixtureChecksum='valid'
function global:Invoke-WebRequest {
 param([switch]$UseBasicParsing,[string]$Uri,[string]$OutFile)
 $Arch=if($env:PROCESSOR_ARCHITEW6432){$env:PROCESSOR_ARCHITEW6432}else{$env:PROCESSOR_ARCHITECTURE}
 $Arch=if($Arch -eq 'ARM64'){'arm64'}else{'amd64'}
 $Asset="tofa_v0.0.0-test_windows_$Arch.exe"
 if ($Uri -notin @("$env:TOFA_RELEASE_BASE_URL/$Asset", "$env:TOFA_RELEASE_BASE_URL/SHA256SUMS")) {throw "Unexpected download: $Uri"}
 if($Uri.EndsWith('/SHA256SUMS')){
  $Hash=[Security.Cryptography.SHA256]::Create()
  try{$Sum=([BitConverter]::ToString($Hash.ComputeHash($global:TofaFixtureBytes))).Replace('-','').ToLower()}finally{$Hash.Dispose()}
  $Line="$Sum  $Asset"
  $Content=switch($global:TofaFixtureChecksum) {
   'valid' {$Line}
   'missing' {''}
   'other asset' {"$Sum  another-binary.exe"}
   'duplicate' {"$Line`n$Line"}
   'malformed duplicate' {"$Line`ninvalid  $Asset"}
   'incorrect' {('0'*64)+"  $Asset"}
   'extra fields' {"$Line unexpected"}
   default {throw 'Unknown checksum fixture'}
  }
  Set-Content -LiteralPath $OutFile -Value $Content -Encoding Ascii
 }else{
  if($global:TofaFixtureCorrupt){[IO.File]::WriteAllText($OutFile,'corrupt')}else{[IO.File]::WriteAllBytes($OutFile,$global:TofaFixtureBytes)}
 }
}
try {
 $env:LOCALAPPDATA=$Temp;$env:TOFA_INSTALL_DIR=$null;$env:TOFA_RELEASE_BASE_URL='https://fixture.invalid/releases/download/v0.0.0-test'
 & (Join-Path $Scripts 'install.ps1') -Version v0.0.0-test -NoModifyPath
 $Binary=Join-Path $Temp 'tofa\install\bin\tofa.exe'
 Assert (Test-Path -LiteralPath $Binary) 'Binary not installed'
 $Manifest=Join-Path $Temp 'tofa/install/.tofa-install'
 $OriginalManifest=Get-Content -Raw -LiteralPath $Manifest
 $global:TofaFixtureBytes=[Text.Encoding]::UTF8.GetBytes('replacement executable')
 foreach($Case in @('missing','other asset','duplicate','malformed duplicate','incorrect','extra fields')) {
  $global:TofaFixtureChecksum=$Case;$Rejected=$false
  try{& (Join-Path $Scripts 'install.ps1') -Version v0.0.0-test -NoModifyPath}catch{
   Assert ($_.Exception.Message -match 'checksum') "Unexpected failure for ${Case}: $_"
   $Rejected=$true
  }
  Assert $Rejected "Invalid checksum accepted: $Case"
  Assert ((Get-Content -Raw -LiteralPath $Binary) -eq 'fixture executable') "Failed upgrade changed binary: $Case"
  Assert ((Get-Content -Raw -LiteralPath $Manifest) -eq $OriginalManifest) "Failed upgrade changed ownership: $Case"
 }
 $global:TofaFixtureChecksum='valid'
 $global:TofaFixtureBytes=[Text.Encoding]::UTF8.GetBytes('fixture executable')
 & (Join-Path $Scripts 'install.ps1') -Version v0.0.0-test -NoModifyPath
 $global:TofaFixtureCorrupt=$true;$Rejected=$false
 try{& (Join-Path $Scripts 'install.ps1') -Version v0.0.0-test -NoModifyPath}catch{$Rejected=$true}
 Assert $Rejected 'Corrupt upgrade accepted'
 Assert ((Get-Content -Raw -LiteralPath $Binary) -eq 'fixture executable') 'Failed upgrade changed binary'
 if ($InstallerOnly) {Write-Host 'Offline PowerShell installer checks passed (uninstall not exercised).'; return}
 $Config=Join-Path $Temp 'tofa'
 Set-Content -LiteralPath (Join-Path $Config 'config.yml') -Value 'version: 1'
 Set-Content -LiteralPath (Join-Path $Config 'credentials.yml') -Value 'fixture: synthetic-key'
 Set-Content -LiteralPath (Join-Path $Config 'unrelated.txt') -Value 'keep'
 Set-Content -LiteralPath (Join-Path $Temp 'tofa/install/unrelated.txt') -Value 'keep installation neighbor'
 & (Join-Path $Scripts 'uninstall.ps1')
 Assert (!(Test-Path -LiteralPath $Binary)) 'Binary retained'
 Assert (Test-Path -LiteralPath (Join-Path $Config 'credentials.yml')) 'Default uninstall deleted credential'
 $global:TofaFixtureCorrupt=$false
 & (Join-Path $Scripts 'install.ps1') -Version v0.0.0-test -NoModifyPath
 Assert (Test-Path -LiteralPath $Binary) 'Retained installation cannot be reused'
 Assert ((Get-Content -Raw -LiteralPath (Join-Path $Temp 'tofa/install/unrelated.txt')).Trim() -eq 'keep installation neighbor') 'Reinstall changed unrelated file'
 Assert ((Get-Content -Raw -LiteralPath (Join-Path $Config 'credentials.yml')).Trim() -eq 'fixture: synthetic-key') 'Reinstall changed saved credential'
 & (Join-Path $Scripts 'uninstall.ps1') -Purge
 Assert (!(Test-Path -LiteralPath (Join-Path $Config 'credentials.yml'))) 'Purge retained credential'
 Assert (Test-Path -LiteralPath (Join-Path $Config 'unrelated.txt')) 'Purge deleted unrelated file'
 Assert (Test-Path -LiteralPath (Join-Path $Temp 'tofa/install/unrelated.txt')) 'Purge deleted installation neighbor'
 Assert (!(Test-Path -LiteralPath $Manifest)) 'Purge retained ownership marker'
 Write-Host 'Offline Windows installer lifecycle passed.'
}finally{
 Remove-Item Function:\Invoke-WebRequest
 Remove-Variable TofaFixtureBytes,TofaFixtureCorrupt,TofaFixtureChecksum -Scope Global
 $env:LOCALAPPDATA=$OriginalLocal;$env:TOFA_RELEASE_BASE_URL=$OriginalBase;$env:TOFA_INSTALL_DIR=$OriginalRoot
 Remove-Item -LiteralPath $Temp -Recurse -Force
}

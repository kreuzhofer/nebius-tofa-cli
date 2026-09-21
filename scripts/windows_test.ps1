# Offline lifecycle test. Uses a temporary LOCALAPPDATA and no PATH/keyring changes.
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
function global:Invoke-WebRequest {
 param([switch]$UseBasicParsing,[string]$Uri,[string]$OutFile)
 if($Uri.EndsWith('/SHA256SUMS')){
  $Hash=[Security.Cryptography.SHA256]::Create()
  try{$Sum=([BitConverter]::ToString($Hash.ComputeHash($global:TofaFixtureBytes))).Replace('-','').ToLower()}finally{$Hash.Dispose()}
  $Arch=if($env:PROCESSOR_ARCHITEW6432){$env:PROCESSOR_ARCHITEW6432}else{$env:PROCESSOR_ARCHITECTURE}
  $Arch=if($Arch -eq 'ARM64'){'arm64'}else{'amd64'}
  Set-Content -LiteralPath $OutFile -Value "$Sum  tofa_v0.0.0-test_windows_$Arch.exe" -Encoding Ascii
 }else{
  if($global:TofaFixtureCorrupt){[IO.File]::WriteAllText($OutFile,'corrupt')}else{[IO.File]::WriteAllBytes($OutFile,$global:TofaFixtureBytes)}
 }
}
try {
 $env:LOCALAPPDATA=$Temp;$env:TOFA_INSTALL_DIR=$null;$env:TOFA_RELEASE_BASE_URL='https://fixture.invalid'
 & (Join-Path $Scripts 'install.ps1') -Version v0.0.0-test -NoModifyPath
 $Binary=Join-Path $Temp 'tofa\install\bin\tofa.exe'
 Assert (Test-Path -LiteralPath $Binary) 'Binary not installed'
 & (Join-Path $Scripts 'install.ps1') -Version v0.0.0-test -NoModifyPath
 $global:TofaFixtureCorrupt=$true;$Rejected=$false
 try{& (Join-Path $Scripts 'install.ps1') -Version v0.0.0-test -NoModifyPath}catch{$Rejected=$true}
 Assert $Rejected 'Corrupt upgrade accepted'
 Assert ((Get-Content -Raw -LiteralPath $Binary) -eq 'fixture executable') 'Failed upgrade changed binary'
 $Config=Join-Path $Temp 'tofa'
 Set-Content -LiteralPath (Join-Path $Config 'config.yml') -Value 'version: 1'
 Set-Content -LiteralPath (Join-Path $Config 'credentials.yml') -Value 'fixture: synthetic-key'
 Set-Content -LiteralPath (Join-Path $Config 'unrelated.txt') -Value 'keep'
 & (Join-Path $Scripts 'uninstall.ps1')
 Assert (!(Test-Path -LiteralPath $Binary)) 'Binary retained'
 Assert (Test-Path -LiteralPath (Join-Path $Config 'credentials.yml')) 'Default uninstall deleted credential'
 & (Join-Path $Scripts 'uninstall.ps1') -Purge
 Assert (!(Test-Path -LiteralPath (Join-Path $Config 'credentials.yml'))) 'Purge retained credential'
 Assert (Test-Path -LiteralPath (Join-Path $Config 'unrelated.txt')) 'Purge deleted unrelated file'
 Write-Host 'Offline Windows installer lifecycle passed.'
}finally{
 Remove-Item Function:\Invoke-WebRequest
 Remove-Variable TofaFixtureBytes,TofaFixtureCorrupt -Scope Global
 $env:LOCALAPPDATA=$OriginalLocal;$env:TOFA_RELEASE_BASE_URL=$OriginalBase;$env:TOFA_INSTALL_DIR=$OriginalRoot
 Remove-Item -LiteralPath $Temp -Recurse -Force
}

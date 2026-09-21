# Invoke the unchanged, checksum-verified release installer using local assets.
# This download boundary is limited to this child PowerShell scope and two exact URLs.
param([Parameter(Mandatory=$true)][string]$Directory,
      [Parameter(Mandatory=$true)][string]$Version,
      [Parameter(Mandatory=$true)][string]$Asset)
$ErrorActionPreference='Stop'
Set-StrictMode -Version Latest
$Directory=(Resolve-Path -LiteralPath $Directory).Path
$env:TOFA_RELEASE_BASE_URL='https://verified-local.invalid/'+$Version
function Invoke-WebRequest {
 param([switch]$UseBasicParsing,[string]$Uri,[string]$OutFile)
 $Name=if($Uri -ceq "$env:TOFA_RELEASE_BASE_URL/$Asset"){$Asset}
       elseif($Uri -ceq "$env:TOFA_RELEASE_BASE_URL/SHA256SUMS"){'SHA256SUMS'}
       else {throw 'Unexpected verified-local download request'}
 Copy-Item -LiteralPath (Join-Path $Directory $Name) -Destination $OutFile
}
& (Join-Path $Directory 'install.ps1') -Version $Version

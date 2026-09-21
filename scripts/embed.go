// Package scripts embeds the recovery scripts so CLI uninstall needs no download.
package scripts

import _ "embed"

//go:embed uninstall.sh
var UninstallSH string

//go:embed uninstall.ps1
var UninstallPS string

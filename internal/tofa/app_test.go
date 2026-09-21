package tofa_test

import (
	"bytes"
	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
	"strings"
	"testing"
)

func TestVersionNeedsNeitherCredentialsNorClient(t *testing.T) {
	var out bytes.Buffer
	app := tofa.App{Out: &out, Version: "prototype-test", Dir: t.TempDir()}
	if err := app.Run([]string{"--version"}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "tofa prototype-test" {
		t.Fatalf("unexpected version: %s", out.String())
	}
}

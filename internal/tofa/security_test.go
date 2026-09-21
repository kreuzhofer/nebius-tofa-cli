package tofa_test

import (
	"bytes"
	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogNeverFollowsRedirectOrPrintsResponseBody(t *testing.T) {
	v := &vault{values: map[string]string{}}
	dir := t.TempDir()
	s := tofa.Store{Dir: dir, Vault: v}
	if err := s.Login("project", "synthetic-token", "keyring"); err != nil {
		t.Fatal(err)
	}
	followed := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed = true }))
	defer destination.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer server.Close()
	var out bytes.Buffer
	app := tofa.App{Out: &out, Dir: dir, Vault: v, Endpoint: server.URL, HTTP: server.Client()}
	err := app.Run([]string{"models"})
	if err == nil || followed {
		t.Fatal("redirect accepted", err)
	}
	if strings.Contains(out.String(), "synthetic-token") || strings.Contains(err.Error(), "synthetic-token") {
		t.Fatal("token leaked")
	}
}

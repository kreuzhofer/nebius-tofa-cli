package tofa_test

import (
	"github.com/kreuzhofer/nebius-tofa-cli/internal/tofa"
	"golang.org/x/sys/windows"
	"path/filepath"
	"testing"
)

func TestWindowsRejectsEveryoneAccessToPlaintextKey(t *testing.T) {
	dir := t.TempDir()
	s := tofa.Store{Dir: dir, Vault: &vault{values: map[string]string{}}}
	if err := s.Login("project", "fixture-key", "file"); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "credentials.yml")
	if err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Credentials(); err == nil {
		t.Fatal("accepted an Everyone-accessible key")
	}
}

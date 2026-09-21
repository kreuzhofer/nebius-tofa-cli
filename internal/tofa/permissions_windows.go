package tofa

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"unsafe"
)

func ownerDescriptor() (*windows.SECURITY_DESCRIPTOR, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")")
}
func protect(path string, dir bool) error {
	sd, err := ownerDescriptor()
	if err != nil {
		return err
	}
	acl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil)
}
func checkPrivate(path string, info os.FileInfo) error {
	got, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}

	control, _, err := got.Control()
	if err != nil {
		return err
	}
	acl, _, err := got.DACL()
	if err != nil {
		return err
	}
	if control&windows.SE_DACL_PROTECTED == 0 || acl == nil || acl.AceCount != 1 {
		return errors.New("file must have a protected owner-only ACL")
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err = windows.GetAce(acl, 0, &ace); err != nil {
		return err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	// Windows stores the variable-length SID starting at SidStart; this is the
	// documented ACCESS_ALLOWED_ACE layout, not Go-owned memory to dereference freely.
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || !sid.IsValid() || !sid.Equals(user.User.Sid) {
		return errors.New("file ACL grants access beyond the current user")
	}
	return nil
}

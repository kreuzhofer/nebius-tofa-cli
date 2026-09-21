package tofa

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
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
	want, err := ownerDescriptor()
	if err != nil {
		return err
	}
	if got.String() != want.String() {
		return errors.New("config/credential file ACL is not the expected private tofa ACL; restore owner-only permissions")
	}
	return nil
}

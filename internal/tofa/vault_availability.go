package tofa

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"

	"github.com/godbus/dbus/v5"
	"github.com/zalando/go-keyring"
)

// Availability probes a fresh, nonsecret reference without creating a credential.
// A missing item establishes access to the facility, not successful future writes.
// In particular, a failed write must never trigger automatic file storage.
func (NativeVault) Availability() error {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return errors.New("could not prepare credential vault availability check")
	}
	_, err := keyring.Get(Service, "availability-"+hex.EncodeToString(id[:]))
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if errors.Is(err, keyring.ErrUnsupportedPlatform) {
		return ErrVaultAbsent
	}
	var busErr *dbus.Error
	if errors.As(err, &busErr) && busErr.Name == "org.freedesktop.DBus.Error.ServiceUnknown" {
		return ErrVaultAbsent
	}
	var busValue dbus.Error
	if errors.As(err, &busValue) && busValue.Name == "org.freedesktop.DBus.Error.ServiceUnknown" {
		return ErrVaultAbsent
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Path == "/usr/bin/security" && errors.Is(err, os.ErrNotExist) {
		return ErrVaultAbsent
	}
	// Do not expose raw OS errors or interpret a missing session bus, a failed
	// activation, a locked collection, or denied access as an absent vault.
	return errors.New("credential vault unavailable, locked, access denied, or availability uncertain")
}

//go:build !windows

package tofa

func preparePickerOutput() (func() error, error) {
	return func() error { return nil }, nil
}

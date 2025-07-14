//go:build linux || darwin

package priority

func LowPriority(pid int) error {
	return nil
}

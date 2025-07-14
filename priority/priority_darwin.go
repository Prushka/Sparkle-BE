//go:build darwin

package priority

func LowPriority(pid int) error {
	return nil
}

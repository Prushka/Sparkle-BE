//go:build linux || darwin

package main

func lowPriority(pid int) error {
	return nil
}

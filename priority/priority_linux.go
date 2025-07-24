//go:build linux

package priority

import (
	"Sparkle/discord"
	"syscall"
)

// Priority constants for Linux
const (
	PRIO_PROCESS = 0  // Process priority type
	LOW_PRIORITY = 16 // Nice value for low priority (higher nice = lower priority)
)

func setPriorityLinux(pid int, priority int) error {
	return syscall.Setpriority(PRIO_PROCESS, pid, priority)
}

func LowPriority(pid int) error {
	err := setPriorityLinux(pid, LOW_PRIORITY)
	if err != nil {
		discord.Errorf("error setting low priority for pid %d: %v", pid, err)
		return err
	}
	return nil
}

//go:build darwin

package priority

import (
	"Sparkle/discord"
	"syscall"
)

// Priority constants for macOS
const (
	PRIO_PROCESS = 0  // Process priority type
	LOW_PRIORITY = 16 // Nice value for low priority (higher nice = lower priority)
)

func setPriorityDarwin(pid int, priority int) error {
	return syscall.Setpriority(PRIO_PROCESS, pid, priority)
}

func LowPriority(pid int) error {
	err := setPriorityDarwin(pid, LOW_PRIORITY)
	if err != nil {
		discord.Errorf("error setting low priority for pid %d: %v", pid, err)
		return err
	}
	return nil
}

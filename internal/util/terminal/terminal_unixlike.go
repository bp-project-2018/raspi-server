// +build linux darwin dragonfly freebsd netbsd openbsd

package terminal

import (
	"golang.org/x/sys/unix"
	"syscall"
)

// MakeCbreak puts the terminal connected to the given file descriptor into
// cbreak (rare) mode and returns the previous state of the terminal so that it
// can be restored. In cbreak mode line buffering and input echoing are turned
// off, but keystrokes like abort are still processed by the terminal.
func MakeCbreak(fd int) (*State, error) {
	old, err := GetState(fd)
	if err != nil {
		return nil, err
	}

	termios, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		return nil, err
	}

	termios.Lflag &^= syscall.ECHO | syscall.ICANON
	termios.Cc[syscall.VMIN] = 1
	termios.Cc[syscall.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, termios); err != nil {
		return nil, err
	}

	return old, nil
}

func EnableVirtualTerminalProcessing(fd int) (*State, error) {
	return GetState(fd)
}

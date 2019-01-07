// +build windows

package terminal

import (
	"golang.org/x/sys/windows"
)

// MakeCbreak puts the terminal connected to the given file descriptor into
// cbreak (rare) mode and returns the previous state of the terminal so that
// it can be restored. In cbreak mode line buffering and input echoing are
// turned off, but keystrokes like abort are still processed by the terminal.
func MakeCbreak(fd int) (*State, error) {
	old, err := GetState(fd)
	if err != nil {
		return nil, err
	}
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(fd), &mode); err != nil {
		return nil, err
	}
	mode &^= (windows.ENABLE_ECHO_INPUT | windows.ENABLE_LINE_INPUT)
	if err := windows.SetConsoleMode(windows.Handle(fd), mode); err != nil {
		return nil, err
	}
	return old, nil
}

func EnableVirtualTerminalProcessing(fd int) (*State, error) {
	old, err := GetState(fd)
	if err != nil {
		return nil, err
	}
	var mode uint32
	if err := windows.GetConsoleMode(windows.Handle(fd), &mode); err != nil {
		return nil, err
	}
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if err := windows.SetConsoleMode(windows.Handle(fd), mode); err != nil {
		return nil, err
	}
	return old, nil
}

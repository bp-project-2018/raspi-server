package terminal

import (
	"golang.org/x/crypto/ssh/terminal"
)

type State terminal.State

func IsTerminal(fd int) bool {
	return terminal.IsTerminal(fd)
}

func MakeRaw(fd int) (*State, error) {
	state, err := terminal.MakeRaw(fd)
	return (*State)(state), err
}

func GetState(fd int) (*State, error) {
	state, err := terminal.GetState(fd)
	return (*State)(state), err
}

func Restore(fd int, state *State) error {
	return terminal.Restore(fd, (*terminal.State)(state))
}

func GetSize(fd int) (width, height int, err error) {
	return terminal.GetSize(fd)
}

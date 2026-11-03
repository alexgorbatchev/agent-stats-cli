//go:build windows

package main

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// enableANSI turns on virtual terminal processing for w's console so it handles ANSI
// escape sequences, and reports whether that succeeded.
func enableANSI(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	handle := windows.Handle(f.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(handle, &mode); err != nil {
		return false
	}
	return windows.SetConsoleMode(handle, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING) == nil
}

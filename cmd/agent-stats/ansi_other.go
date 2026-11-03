//go:build !windows

package main

import "io"

// enableANSI reports whether w's terminal handles ANSI escape sequences, which every
// supported terminal outside Windows does.
func enableANSI(io.Writer) bool { return true }

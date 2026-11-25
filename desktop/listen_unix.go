//go:build !windows

package desktop

import "syscall"

const errAddressInUse = syscall.EADDRINUSE

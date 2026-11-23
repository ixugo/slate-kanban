package desktop

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

func systemLanguages() []string {
	// LOCALE_NAME_MAX_LENGTH includes the terminating NUL.
	var name [85]uint16
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")
	if n, _, _ := proc.Call(uintptr(unsafe.Pointer(&name[0])), uintptr(len(name))); n == 0 {
		return nil
	}
	return []string{windows.UTF16ToString(name[:])}
}

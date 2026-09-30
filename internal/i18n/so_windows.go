//go:build windows

package i18n

import (
	"syscall"
	"unsafe"
)

// idiomaSO pregunta a Windows el idioma del usuario ("es-AR", "en-US").
func idiomaSO() string {
	k := syscall.NewLazyDLL("kernel32.dll")
	f := k.NewProc("GetUserDefaultLocaleName")
	buf := make([]uint16, 85)
	r, _, _ := f.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

// Package fb holds the Miyoo Mini framebuffer basics shared by the full UI
// (ui) and small helpers that draw over other programs, like the
// MiniScale badge, which shouldn't carry the UI's fonts in memory.
package fb

import (
	"os"
	"syscall"
	"unsafe"
)

// The screen size in pixels.
const (
	W = 640
	H = 480
)

// VarScreenInfo is fb_var_screeninfo from linux/fb.h (only the fields used
// here are named).
type VarScreenInfo struct {
	Xres, Yres               uint32
	XresVirtual, YresVirtual uint32
	Xoffset, Yoffset         uint32
	BitsPerPixel, Grayscale  uint32
	Rest                     [32]uint32 // bitfields, timings, reserved
}

// Framebuffer ioctl requests (linux/fb.h).
const (
	IOGetVScreenInfo = 0x4600
	IOPanDisplay     = 0x4606
	IOWaitForVsync   = 0x40044620
)

// Ioctl issues the framebuffer request req on f with arg.
func Ioctl(f *os.File, req uintptr, arg unsafe.Pointer) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), req, uintptr(arg))
	if e != 0 {
		return e
	}
	return nil
}

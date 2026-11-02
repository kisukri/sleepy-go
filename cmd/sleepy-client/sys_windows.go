//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetForegroundWindow  = user32.NewProc("GetForegroundWindow")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetLastInputInfo     = user32.NewProc("GetLastInputInfo")
	procGetTickCount         = kernel32.NewProc("GetTickCount")
	procGetSystemPowerStatus = kernel32.NewProc("GetSystemPowerStatus")
)

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

type systemPowerStatus struct {
	ACLineStatus        byte
	BatteryFlag         byte
	BatteryLifePercent  byte
	SystemStatusFlag    byte
	BatteryLifeTime     uint32
	BatteryFullLifeTime uint32
}

type winSampler struct{}

func newSampler() (Sampler, error) { return winSampler{}, nil }

func (winSampler) Sample() (Sample, error) {
	return Sample{Title: foregroundTitle(), Idle: idleTime(), Battery: battery()}, nil
}

func foregroundTitle() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ""
	}
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

// both are 32bit ms ticks, uint32 minus survives the 49 day wrap
func idleTime() time.Duration {
	info := lastInputInfo{cbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	if ok, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info))); ok == 0 {
		return 0
	}
	now, _, _ := procGetTickCount.Call()
	return time.Duration(uint32(now)-info.dwTime) * time.Millisecond
}

func battery() *Battery {
	var s systemPowerStatus
	if ok, _, _ := procGetSystemPowerStatus.Call(uintptr(unsafe.Pointer(&s))); ok == 0 {
		return nil
	}
	if s.BatteryFlag == 128 || s.BatteryFlag == 255 || s.BatteryLifePercent > 100 {
		return nil
	}
	return &Battery{Percent: int(s.BatteryLifePercent), Charging: s.ACLineStatus == 1}
}

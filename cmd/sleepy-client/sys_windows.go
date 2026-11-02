//go:build windows

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	versionDLL                     = syscall.NewLazyDLL("version.dll")
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procGetWindowTextLengthW       = user32.NewProc("GetWindowTextLengthW")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procGetLastInputInfo           = user32.NewProc("GetLastInputInfo")
	procGetTickCount               = kernel32.NewProc("GetTickCount")
	procGetSystemPowerStatus       = kernel32.NewProc("GetSystemPowerStatus")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procGetFileVersionInfoSizeW    = versionDLL.NewProc("GetFileVersionInfoSizeW")
	procGetFileVersionInfoW        = versionDLL.NewProc("GetFileVersionInfoW")
	procVerQueryValueW             = versionDLL.NewProc("VerQueryValueW")
)

const processQueryLimitedInformation = 0x1000

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

type winSampler struct {
	names map[string]string // exe path -> program name
}

func newSampler() (Sampler, error) { return &winSampler{names: map[string]string{}}, nil }

func (w *winSampler) Sample() (Sample, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	title := windowTitle(hwnd)
	return Sample{Title: title, App: w.appName(hwnd, title), Idle: idleTime(), Battery: battery()}, nil
}

func windowTitle(hwnd uintptr) string {
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

func (w *winSampler) appName(hwnd uintptr, title string) string {
	path := processPath(hwnd)
	if path == "" {
		return ""
	}
	base := filepath.Base(path)
	// Settings, Calculator etc. all live in this host; their title is the app name
	if strings.EqualFold(base, "ApplicationFrameHost.exe") {
		return title
	}
	if name, ok := w.names[path]; ok {
		return name
	}
	name := fileDescription(path)
	if name == "" {
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}
	w.names[path] = name
	return name
}

func processPath(hwnd uintptr) string {
	if hwnd == 0 {
		return ""
	}
	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ""
	}
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)
	buf := make([]uint16, 1024)
	n := uint32(len(buf))
	if ok, _, _ := procQueryFullProcessImageNameW.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&n))); ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

// the "File description" shown in the exe's properties, e.g. "Microsoft Edge"
func fileDescription(path string) string {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ""
	}
	size, _, _ := procGetFileVersionInfoSizeW.Call(uintptr(unsafe.Pointer(p)), 0)
	if size == 0 {
		return ""
	}
	data := make([]byte, size)
	if ok, _, _ := procGetFileVersionInfoW.Call(uintptr(unsafe.Pointer(p)), 0, size, uintptr(unsafe.Pointer(&data[0]))); ok == 0 {
		return ""
	}
	query := func(sub string) []uint16 {
		q, _ := syscall.UTF16PtrFromString(sub)
		var v unsafe.Pointer
		var n uint32
		if ok, _, _ := procVerQueryValueW.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(unsafe.Pointer(q)), uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&n))); ok == 0 || v == nil || n == 0 {
			return nil
		}
		return unsafe.Slice((*uint16)(v), n)
	}
	var langs []string
	if tr := query(`\VarFileInfo\Translation`); len(tr) >= 2 {
		langs = append(langs, fmt.Sprintf("%04x%04x", tr[0], tr[1]))
	}
	for _, lang := range append(langs, "040904b0", "040904e4") {
		if v := query(`\StringFileInfo\` + lang + `\FileDescription`); v != nil {
			if s := strings.TrimSpace(syscall.UTF16ToString(v)); s != "" {
				return s
			}
		}
	}
	return ""
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

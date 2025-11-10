//go:build windows

package clipboardwatcher

import (
	"fmt"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	WM_DESTROY         = 0x0002
	WM_QUIT            = 0x0012
	WM_CLIPBOARDUPDATE = 0x031D

	CF_UNICODETEXT = 13
)

// WNDCLASSEXW mirrors the WinAPI struct.
type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

// MSG mirrors the WinAPI MSG struct.
type MSG struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
}

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procAddClipboardFormatListener = user32.NewProc("AddClipboardFormatListener")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procGetClipboardData           = user32.NewProc("GetClipboardData")

	procGlobalLock   = kernel32.NewProc("GlobalLock")
	procGlobalUnlock = kernel32.NewProc("GlobalUnlock")

	procPostQuitMessage = user32.NewProc("PostQuitMessage")
)

// Handler is invoked every time new Unicode text is detected on the clipboard.
type Handler func(text string)

// clipboardHandler holds the user callback.
var clipboardHandler Handler

// utf16PtrToString converts a *uint16 (null-terminated UTF-16) to Go string.
func utf16PtrToString(ptr *uint16) string {
	if ptr == nil {
		return ""
	}
	u := (*[1 << 30]uint16)(unsafe.Pointer(ptr))
	n := 0
	for u[n] != 0 {
		n++
	}
	return string(utf16.Decode(u[:n]))
}

// readClipboardText reads CF_UNICODETEXT with retry/backoff around OpenClipboard.
func readClipboardText() (string, error) {
	// Is there Unicode text available?
	ret, _, _ := procIsClipboardFormatAvailable.Call(uintptr(CF_UNICODETEXT))
	if ret == 0 {
		return "", fmt.Errorf("no CF_UNICODETEXT available")
	}

	const maxAttempts = 5
	var (
		opened  bool
		lastErr error
	)

	// Retry OpenClipboard: 10, 20, 40, 80, 160 ms.
	for attempt := 0; attempt < maxAttempts; attempt++ {
		r1, _, err := procOpenClipboard.Call(0)
		if r1 != 0 {
			opened = true
			break
		}
		lastErr = err
		time.Sleep(time.Duration(10*(1<<attempt)) * time.Millisecond)
	}
	if !opened {
		return "", fmt.Errorf("OpenClipboard failed after %d attempts: %v", maxAttempts, lastErr)
	}
	defer procCloseClipboard.Call()

	handle, _, _ := procGetClipboardData.Call(uintptr(CF_UNICODETEXT))
	if handle == 0 {
		return "", fmt.Errorf("GetClipboardData returned null")
	}

	ptr, _, _ := procGlobalLock.Call(handle)
	if ptr == 0 {
		return "", fmt.Errorf("GlobalLock failed")
	}
	defer procGlobalUnlock.Call(handle)

	text := utf16PtrToString((*uint16)(unsafe.Pointer(ptr)))
	return text, nil
}

// wndProc is the window procedure that receives WM_CLIPBOARDUPDATE.
func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CLIPBOARDUPDATE:
		if clipboardHandler != nil {
			if text, err := readClipboardText(); err == nil {
				clipboardHandler(text)
			}
		}
		return 0

	case WM_DESTROY:
		procPostQuitMessage.Call(0)
		return 0

	default:
		ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
		return ret
	}
}

// Watch starts a clipboard watcher and calls handler for each Unicode text change.
// It blocks, running a Windows message loop, until WM_QUIT is received
// (e.g. process exit or someone posts WM_QUIT).
func Watch(handler Handler) error {
	if handler == nil {
		return fmt.Errorf("clipboardwatcher: handler must not be nil")
	}
	clipboardHandler = handler

	// Prepare window class.
	className, err := windows.UTF16PtrFromString("GoClipboardWatcherClass")
	if err != nil {
		return err
	}

	var hInstance windows.Handle
	if err := windows.GetModuleHandleEx(0, nil, &hInstance); err != nil {
		return fmt.Errorf("GetModuleHandleEx failed: %w", err)
	}

	var wcex WNDCLASSEXW
	wcex.CbSize = uint32(unsafe.Sizeof(wcex))
	wcex.LpfnWndProc = windows.NewCallback(wndProc)
	wcex.HInstance = uintptr(hInstance)
	wcex.LpszClassName = className

	r, _, e := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcex)))
	if r == 0 {
		return fmt.Errorf("RegisterClassExW failed: %v", e)
	}

	// Create an invisible window.
	hwnd, _, e := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)), // lpClassName
		0,                                  // lpWindowName
		0,                                  // dwStyle
		0, 0, 0, 0,                         // x, y, width, height
		0, // hWndParent
		0, // hMenu
		uintptr(hInstance),
		0, // lpParam
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed: %v", e)
	}

	// Register for clipboard updates.
	if r, _, e = procAddClipboardFormatListener.Call(hwnd); r == 0 {
		return fmt.Errorf("AddClipboardFormatListener failed: %v", e)
	}

	// Message loop.
	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) == -1 {
			return fmt.Errorf("GetMessageW error")
		}
		if ret == 0 { // WM_QUIT
			return nil
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

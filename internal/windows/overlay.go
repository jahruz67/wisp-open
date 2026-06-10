//go:build windows

// ============================================================
// WINDOWS-ONLY FILE — Native Windows overlay window using
// Win32 API directly (CreateWindowEx, GDI drawing, etc.).
// This shows a pill-shaped overlay on screen during recording.
// The Linux equivalent is internal/linux/overlay.go (no-op).
// ============================================================

package windows

import (
	"math"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                         = syscall.NewLazyDLL("user32.dll")
	gdi32                          = syscall.NewLazyDLL("gdi32.dll")
	procCreateWindowEx             = user32.NewProc("CreateWindowExW")
	procDefWindowProc              = user32.NewProc("DefWindowProcW")
	procDispatchMessage            = user32.NewProc("DispatchMessageW")
	procPeekMessage                = user32.NewProc("PeekMessageW")
	procRegisterClassEx            = user32.NewProc("RegisterClassExW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procUpdateWindow               = user32.NewProc("UpdateWindow")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procCreateSolidBrush           = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW                = gdi32.NewProc("CreateFontW")
	procSelectObject               = gdi32.NewProc("SelectObject")
	procDeleteObject               = gdi32.NewProc("DeleteObject")
	procSetBkMode                  = gdi32.NewProc("SetBkMode")
	procSetTextColor               = gdi32.NewProc("SetTextColor")
	procDrawTextW                  = user32.NewProc("DrawTextW")
	procPostMessage                = user32.NewProc("PostMessageW")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
	procLoadCursor                 = user32.NewProc("LoadCursorW")
	procCreatePen                  = gdi32.NewProc("CreatePen")
	procEllipse                    = gdi32.NewProc("Ellipse")
	procCreateRoundRectRgn         = gdi32.NewProc("CreateRoundRectRgn")
	procSetWindowRgn               = user32.NewProc("SetWindowRgn")
	procFillRgn                    = gdi32.NewProc("FillRgn")
	procCreateCompatibleDC         = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap     = gdi32.NewProc("CreateCompatibleBitmap")
	procBitBlt                     = gdi32.NewProc("BitBlt")
	procDeleteDC                   = gdi32.NewProc("DeleteDC")
)

const (
	WS_POPUP          = 0x80000000
	WS_EX_LAYERED     = 0x00080000
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_TRANSPARENT = 0x00000020
	SW_SHOW           = 5
	SW_HIDE           = 0
	LWA_ALPHA         = 0x00000002
	SM_CXSCREEN       = 0
	SM_CYSCREEN       = 1
	TRANSPARENT_BK    = 1
	DT_CENTER         = 0x00000001
	DT_VCENTER        = 0x00000004
	DT_SINGLELINE     = 0x00000020
	WM_PAINT          = 0x000F
	WM_DESTROY        = 0x0002
	WM_CLOSE          = 0x0010
	IDC_ARROW         = 32512
	PM_REMOVE         = 0x0001
	PS_SOLID          = 0
	WM_ERASEBKGND     = 0x0014
)

const (
	COLOR_BG_DARK    = 0x1A0F0B // Deep dark background (BGR for #0b0f1a)
	COLOR_MIC_RED    = 0x4545FF // Red for recording (BGR)
	COLOR_MIC_GREEN  = 0x50C850 // Green for ready
	COLOR_MIC_ORANGE = 0x00A5FF // Orange for processing
	COLOR_WHITE      = 0xFFFFFF
	COLOR_GRAY       = 0x808080
)

// Overlay manages a transparent overlay window
type Overlay struct {
	hwnd           syscall.Handle
	text           string
	isShowing      bool
	running        bool
	closed         int32 // atomic: 1 = Close() has been called
	mu             sync.RWMutex
	stopCh         chan struct{}
	bgBrush        syscall.Handle
	barBrushWhite  syscall.Handle // Pre-created GDI brush for white bars (recording animation)
	barBrushOrange syscall.Handle // Pre-created GDI brush for orange bars (transcribing animation)
	hFont          syscall.Handle
	volume         uint64 // atomic: float64 stored via math.Float64bits
	smoothedVolume uint64 // atomic: float64 stored via math.Float64bits
}

var globalOverlay *Overlay

// NewOverlay creates a new overlay instance
func NewOverlay() *Overlay {
	o := &Overlay{
		text:   "Ready",
		stopCh: make(chan struct{}),
	}
	globalOverlay = o
	go o.run()
	return o
}

// Show displays the overlay with the given message
func (o *Overlay) Show(message string) {
	o.mu.Lock()
	o.text = message
	o.isShowing = true
	o.mu.Unlock()

	if o.hwnd != 0 {
		procInvalidateRect.Call(uintptr(o.hwnd), 0, 1)
		procShowWindow.Call(uintptr(o.hwnd), SW_SHOW)
	}
}

// Hide hides the overlay
func (o *Overlay) Hide() {
	o.mu.Lock()
	o.isShowing = false
	o.mu.Unlock()

	if o.hwnd != 0 {
		procShowWindow.Call(uintptr(o.hwnd), SW_HIDE)
	}
}

// SetVolume updates the current audio volume level using lock-free atomics
// so the audio callback thread never blocks on a mutex.
func (o *Overlay) SetVolume(level float64) {
	atomic.StoreUint64(&o.volume, math.Float64bits(level))

	for {
		oldBits := atomic.LoadUint64(&o.smoothedVolume)
		old := math.Float64frombits(oldBits)
		// Stronger smoothing: 10% new value, 90% old value to reduce jitter/flicker
		smoothed := (old * 0.9) + (level * 0.1)
		if atomic.CompareAndSwapUint64(&o.smoothedVolume, oldBits, math.Float64bits(smoothed)) {
			break
		}
	}
}

// Close stops the overlay. Safe to call multiple times.
func (o *Overlay) Close() {
	if !atomic.CompareAndSwapInt32(&o.closed, 0, 1) {
		return // Already closed
	}
	if o.hwnd != 0 {
		procPostMessage.Call(uintptr(o.hwnd), WM_CLOSE, 0, 0)
	}
	if o.bgBrush != 0 {
		procDeleteObject.Call(uintptr(o.bgBrush))
	}
	if o.hFont != 0 {
		procDeleteObject.Call(uintptr(o.hFont))
	}
	if o.barBrushWhite != 0 {
		procDeleteObject.Call(uintptr(o.barBrushWhite))
	}
	if o.barBrushOrange != 0 {
		procDeleteObject.Call(uintptr(o.barBrushOrange))
	}
	close(o.stopCh)
}

func (o *Overlay) run() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	o.running = true
	defer func() { o.running = false }()

	className := syscall.StringToUTF16Ptr("wis-free-v3Overlay")

	var wc wndClassEx
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	wc.lpfnWndProc = syscall.NewCallback(overlayWndProc)
	wc.lpszClassName = className

	// Pre-create resources
	o.bgBrush = syscall.Handle(uintptr(0))
	brushRec, _, _ := procCreateSolidBrush.Call(COLOR_BG_DARK)
	o.bgBrush = syscall.Handle(brushRec)

	whiteRec, _, _ := procCreateSolidBrush.Call(uintptr(COLOR_WHITE))
	o.barBrushWhite = syscall.Handle(whiteRec)

	orangeRec, _, _ := procCreateSolidBrush.Call(uintptr(COLOR_MIC_ORANGE))
	o.barBrushOrange = syscall.Handle(orangeRec)

	fontName := syscall.StringToUTF16Ptr("Segoe UI")
	fontRec, _, _ := procCreateFontW.Call(
		18, 0, 0, 0, 600,
		0, 0, 0, 0, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(fontName)),
	)
	o.hFont = syscall.Handle(fontRec)

	cursor, _, _ := procLoadCursor.Call(0, uintptr(IDC_ARROW))
	wc.hCursor = syscall.Handle(cursor)
	wc.hbrBackground = o.bgBrush

	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))

	// Get screen dimensions
	screenWidth, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	screenHeight, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)

	// Pill dimensions
	width := 120
	height := 46
	x := (int(screenWidth) - width) / 2
	y := int(screenHeight) - height - 80 // 80px from bottom

	hwnd, _, _ := procCreateWindowEx.Call(
		WS_EX_LAYERED|WS_EX_TOPMOST|WS_EX_TOOLWINDOW|WS_EX_TRANSPARENT,
		uintptr(unsafe.Pointer(className)),
		0,
		WS_POPUP,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		0, 0, 0, 0,
	)

	if hwnd == 0 {
		return
	}

	o.hwnd = syscall.Handle(hwnd)

	// Create a rounded region to clip the window (true pill shape, no corners)
	// The corner radius should be half the height for a perfect pill
	rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, uintptr(width+1), uintptr(height+1), uintptr(height), uintptr(height))
	procSetWindowRgn.Call(hwnd, rgn, 1)

	// Set window transparency
	procSetLayeredWindowAttributes.Call(uintptr(hwnd), 0, 240, LWA_ALPHA)
	procShowWindow.Call(uintptr(hwnd), SW_HIDE)
	procUpdateWindow.Call(uintptr(hwnd))

	// Message loop
	ticker := time.NewTicker(16 * time.Millisecond) // ~60 FPS
	defer ticker.Stop()

	var msg msg
	for {
		select {
		case <-o.stopCh:
			return
		case <-ticker.C:
			if o.hwnd != 0 && o.isShowing {
				// Only invalidate if we are in a state that needs animation
				o.mu.RLock()
				needsAnimation := strings.HasPrefix(o.text, "Recording") || strings.HasPrefix(o.text, "Transcribing") || strings.HasPrefix(o.text, "Processing")
				o.mu.RUnlock()

				if needsAnimation {
					procInvalidateRect.Call(uintptr(o.hwnd), 0, 0)
				}
			}
		default:
			ret, _, _ := procPeekMessage.Call(
				uintptr(unsafe.Pointer(&msg)),
				0, 0, 0,
				PM_REMOVE,
			)

			if ret != 0 {
				if msg.message == WM_CLOSE {
					return
				}
				procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
				procDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
			} else {
			o.mu.RLock()
			isShowing := o.isShowing
			text := o.text
			o.mu.RUnlock()

			if isShowing && (strings.HasPrefix(text, "Recording") || strings.HasPrefix(text, "Transcribing")) {
				// Higher resolution sleep only when animating
				time.Sleep(2 * time.Millisecond)
			} else {
				// Reduce wakeups when hidden or showing static text (Ready, errors)
				time.Sleep(50 * time.Millisecond)
			}
			}
		}
	}
}

func overlayWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_ERASEBKGND:
		return 1 // Prevent white flash by handling erase ourselves

	case WM_PAINT:
		var ps paintStruct
		hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))

		// Double buffering: Create a memory DC to draw off-screen first
		memDC, _, _ := procCreateCompatibleDC.Call(hdc)
		memBitmap, _, _ := procCreateCompatibleBitmap.Call(hdc, 120, 46)
		oldBitmap, _, _ := procSelectObject.Call(memDC, memBitmap)

		// Access global state safely
		var text string
		var bgBrush syscall.Handle
		var hFont syscall.Handle
		var volume float64

		if globalOverlay != nil {
			globalOverlay.mu.RLock()
			text = globalOverlay.text
			bgBrush = globalOverlay.bgBrush
			hFont = globalOverlay.hFont
			globalOverlay.mu.RUnlock()
			// Read volume atomically (written by audio thread without lock)
			volume = math.Float64frombits(atomic.LoadUint64(&globalOverlay.smoothedVolume))
		}

		// 1. Clear memory DC with background
		rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, 121, 47, 46, 46)
		if bgBrush != 0 {
			procFillRgn.Call(memDC, rgn, uintptr(bgBrush))
		}
		procDeleteObject.Call(rgn)

		// 2. Draw content to memory DC
		if strings.HasPrefix(text, "Recording") {
			// Draw animated waves for recording
			// Slowed down from 10.0 to 6.0
			t := float64(time.Now().UnixNano()) / 1e9 * 6.0

			barCount := 7 // Reduced from 9 for shorter pill
			barWidth := 4
			barGap := 4
			totalWidth := barCount*barWidth + (barCount-1)*barGap
			startX := (120 - totalWidth) / 2
			centerY := 46 / 2

			// Use colors for waves? The user said "instead of... the color... it's just like waves"
			// But maybe a subtle red pulse is nice? Let's use WHITE as requested.
			barBrush, _, _ := procCreateSolidBrush.Call(uintptr(COLOR_WHITE))

			for i := 0; i < barCount; i++ {
				// Different phases for each bar
				phase := float64(i) * 0.8

				// Base height from animation
				animH := 4.0 * math.Abs(math.Sin(t+phase))

				// Scale height based on real-time volume
				// Boosted sensitivity from 40.0 to 120.0 for better response to normal speech
				volH := volume * 120.0

				// Total height: slow background wave + responsive voice spikes
				h := 6.0 + animH + volH

				// Cap height to 38 (max pill center space)
				if h > 38 {
					h = 38
				}

				x := int32(startX + i*(barWidth+barGap))
				y1 := int32(float64(centerY) - h/2)
				y2 := int32(float64(centerY) + h/2)

				barRgn, _, _ := procCreateRoundRectRgn.Call(uintptr(x), uintptr(y1), uintptr(x+int32(barWidth)), uintptr(y2), 4, 4)
				procFillRgn.Call(memDC, barRgn, barBrush)
				procDeleteObject.Call(barRgn)
			}
			procDeleteObject.Call(barBrush)

		} else if strings.HasPrefix(text, "Transcribing") || strings.HasPrefix(text, "Processing") {
			// Different animation for processing/transcribing (more uniform, pulsing)
			// Slowed down from 5.0 to 3.0
			t := float64(time.Now().UnixNano()) / 1e9 * 3.0

			barCount := 5 // Reduced from 7 for shorter pill
			barWidth := 4
			barGap := 6
			totalWidth := barCount*barWidth + (barCount-1)*barGap
			startX := (120 - totalWidth) / 2
			centerY := 46 / 2

			barBrush, _, _ := procCreateSolidBrush.Call(uintptr(COLOR_MIC_ORANGE)) // Orange for processing

			for i := 0; i < barCount; i++ {
				// Subtler oscillation
				h := 10.0 + 10.0*math.Abs(math.Sin(t+float64(i)*0.3))

				x := int32(startX + i*(barWidth+barGap))
				y1 := int32(float64(centerY) - h/2)
				y2 := int32(float64(centerY) + h/2)

				barRgn, _, _ := procCreateRoundRectRgn.Call(uintptr(x), uintptr(y1), uintptr(x+int32(barWidth)), uintptr(y2), 4, 4)
				procFillRgn.Call(memDC, barRgn, barBrush)
				procDeleteObject.Call(barRgn)
			}
			procDeleteObject.Call(barBrush)
		} else {
			// Show text for errors or "Ready" (though Ready is rarely shown in overlay)
			// This keeps the user informed if something went wrong
			procSetBkMode.Call(memDC, TRANSPARENT_BK)

			textColor := COLOR_WHITE
			if text == "Error" || strings.Contains(text, "Key") || strings.Contains(text, "Err") {
				textColor = COLOR_MIC_RED
			}

			procSetTextColor.Call(memDC, uintptr(textColor))

			if hFont != 0 {
				oldFont, _, _ := procSelectObject.Call(memDC, uintptr(hFont))
				// Centered text across the whole pill since there's no circle anymore
				textRect := rect{0, 0, 120, 46}
				textPtr := syscall.StringToUTF16Ptr(text)
				procDrawTextW.Call(memDC, uintptr(unsafe.Pointer(textPtr)), ^uintptr(0), uintptr(unsafe.Pointer(&textRect)), DT_CENTER|DT_VCENTER|DT_SINGLELINE)
				procSelectObject.Call(memDC, oldFont)
			}
		}

		// 3. Copy everything from memory DC to screen (prevents flicker)
		procBitBlt.Call(hdc, 0, 0, 120, 46, memDC, 0, 0, 0x00CC0020) // SRCCOPY

		// Cleanup memory DC
		procSelectObject.Call(memDC, oldBitmap)
		procDeleteObject.Call(memBitmap)
		procDeleteDC.Call(memDC)

		procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		return 0

	case WM_DESTROY:
		return 0
	}

	ret, _, _ := procDefWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

// Structs for Windows API
type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type msg struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type point struct {
	x, y int32
}

type paintStruct struct {
	hdc         syscall.Handle
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type rect struct {
	left, top, right, bottom int32
}

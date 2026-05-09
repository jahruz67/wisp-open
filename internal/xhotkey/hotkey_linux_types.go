//go:build linux

package hotkey

// Modifier represents a modifier (X11 bitmask; also used when mapping to portal shortcut spec).
type Modifier uint32

// See /usr/include/X11/X.h
const (
	ModCtrl  Modifier = (1 << 2)
	ModShift Modifier = (1 << 0)
	Mod1     Modifier = (1 << 3)
	Mod2     Modifier = (1 << 4)
	Mod3     Modifier = (1 << 5)
	Mod4     Modifier = (1 << 6)
	Mod5     Modifier = (1 << 7)
)

// Key represents a key (X11 keysym values; matches xhotkey / ParseShortcut on Linux).
type Key uint16

// See /usr/include/X11/keysymdef.h
const (
	KeySpace Key = 0x0020
	Key0     Key = 0x0030
	Key1     Key = 0x0031
	Key2     Key = 0x0032
	Key3     Key = 0x0033
	Key4     Key = 0x0034
	Key5     Key = 0x0035
	Key6     Key = 0x0036
	Key7     Key = 0x0037
	Key8     Key = 0x0038
	Key9     Key = 0x0039
	KeyA     Key = 0x0061
	KeyB     Key = 0x0062
	KeyC     Key = 0x0063
	KeyD     Key = 0x0064
	KeyE     Key = 0x0065
	KeyF     Key = 0x0066
	KeyG     Key = 0x0067
	KeyH     Key = 0x0068
	KeyI     Key = 0x0069
	KeyJ     Key = 0x006a
	KeyK     Key = 0x006b
	KeyL     Key = 0x006c
	KeyM     Key = 0x006d
	KeyN     Key = 0x006e
	KeyO     Key = 0x006f
	KeyP     Key = 0x0070
	KeyQ     Key = 0x0071
	KeyR     Key = 0x0072
	KeyS     Key = 0x0073
	KeyT     Key = 0x0074
	KeyU     Key = 0x0075
	KeyV     Key = 0x0076
	KeyW     Key = 0x0077
	KeyX     Key = 0x0078
	KeyY     Key = 0x0079
	KeyZ     Key = 0x007a

	KeyReturn Key = 0xff0d
	KeyEscape Key = 0xff1b
	KeyDelete Key = 0xffff
	KeyTab    Key = 0xff09

	KeyLeft  Key = 0xff51
	KeyRight Key = 0xff53
	KeyUp    Key = 0xff52
	KeyDown  Key = 0xff54

	KeyF1  Key = 0xffbe
	KeyF2  Key = 0xffbf
	KeyF3  Key = 0xffc0
	KeyF4  Key = 0xffc1
	KeyF5  Key = 0xffc2
	KeyF6  Key = 0xffc3
	KeyF7  Key = 0xffc4
	KeyF8  Key = 0xffc5
	KeyF9  Key = 0xffc6
	KeyF10 Key = 0xffc7
	KeyF11 Key = 0xffc8
	KeyF12 Key = 0xffc9
	KeyF13 Key = 0xffca
	KeyF14 Key = 0xffcb
	KeyF15 Key = 0xffcc
	KeyF16 Key = 0xffcd
	KeyF17 Key = 0xffce
	KeyF18 Key = 0xffcf
	KeyF19 Key = 0xffd0
	KeyF20 Key = 0xffd1
)

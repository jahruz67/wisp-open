//go:build windows

package windows

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	win "golang.org/x/sys/windows"
	"wis-free-v3/internal/logger"
)

var (
	mediaOLE              = syscall.NewLazyDLL("ole32.dll")
	mediaCoInitialize     = mediaOLE.NewProc("CoInitializeEx")
	mediaCoUninitialize   = mediaOLE.NewProc("CoUninitialize")
	mediaCoCreate         = mediaOLE.NewProc("CoCreateInstance")
	mediaCoTaskMemFree    = mediaOLE.NewProc("CoTaskMemFree")
	deviceEnumeratorClass = win.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	deviceEnumeratorIID   = win.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	endpointVolumeIID     = win.GUID{Data1: 0x5CDF2C82, Data2: 0x841E, Data3: 0x4546, Data4: [8]byte{0x97, 0x22, 0x0C, 0xF7, 0x40, 0x78, 0x22, 0x9A}}
)

type mediaCOM struct {
	vtable *[16]uintptr
}

//go:uintptrescapes
func (object *mediaCOM) call(method int, args ...uintptr) error {
	result, _, _ := syscall.SyscallN(object.vtable[method], append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)...)
	return mediaHRESULT(result)
}

func mediaHRESULT(result uintptr) error {
	if int32(result) < 0 {
		return fmt.Errorf("audio COM call failed: 0x%08X", uint32(result))
	}
	return nil
}

// Keep COM objects on their owning thread. Resolve the saved endpoint ID when
// restoring, even if the default output device has changed during recording.
func withOutputVolume(deviceID string, change func(*mediaCOM, string) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	result, _, _ := mediaCoInitialize.Call(0, 0) // COINIT_MULTITHREADED
	if uint32(result) != 0x80010106 {            // RPC_E_CHANGED_MODE: COM is already initialized.
		if err := mediaHRESULT(result); err != nil {
			return err
		}
		defer mediaCoUninitialize.Call()
	}
	var enumerator *mediaCOM
	result, _, _ = mediaCoCreate.Call(uintptr(unsafe.Pointer(&deviceEnumeratorClass)), 0, 1,
		uintptr(unsafe.Pointer(&deviceEnumeratorIID)), uintptr(unsafe.Pointer(&enumerator)))
	if err := mediaHRESULT(result); err != nil {
		return err
	}
	defer enumerator.call(2) // IUnknown.Release
	var device *mediaCOM
	if deviceID == "" {
		if err := enumerator.call(4, 0, 1, uintptr(unsafe.Pointer(&device))); err != nil { // eRender, eMultimedia
			return err
		}
	} else {
		id, err := win.UTF16PtrFromString(deviceID)
		if err != nil {
			return err
		}
		if err := enumerator.call(5, uintptr(unsafe.Pointer(id)), uintptr(unsafe.Pointer(&device))); err != nil {
			return err
		}
	}
	defer device.call(2)
	var id *uint16
	if err := device.call(5, uintptr(unsafe.Pointer(&id))); err != nil { // IMMDevice.GetId
		return err
	}
	defer mediaCoTaskMemFree.Call(uintptr(unsafe.Pointer(id)))
	var volume *mediaCOM
	if err := device.call(3, uintptr(unsafe.Pointer(&endpointVolumeIID)), 1, 0, uintptr(unsafe.Pointer(&volume))); err != nil {
		return err
	}
	defer volume.call(2)
	return change(volume, win.UTF16PtrToString(id))
}

// MuteOutput mutes playback without changing the volume setting. The returned
// function restores the original device's previous mute state.
func MuteOutput() func() {
	var original int32
	var deviceID string
	err := withOutputVolume("", func(volume *mediaCOM, id string) error {
		deviceID = id
		if err := volume.call(15, uintptr(unsafe.Pointer(&original))); err != nil { // GetMute
			return err
		}
		return volume.call(14, 1, 0) // SetMute(TRUE)
	})
	if err != nil {
		logger.Error("Failed to mute output: %v", err)
		return nil
	}
	return func() {
		err := withOutputVolume(deviceID, func(volume *mediaCOM, _ string) error {
			return volume.call(14, uintptr(original), 0) // SetMute
		})
		if err != nil {
			logger.Error("Failed to restore output mute state: %v", err)
		}
	}
}

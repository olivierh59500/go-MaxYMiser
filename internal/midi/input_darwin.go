//go:build darwin

package midi

import (
	"fmt"
	"github.com/ebitengine/purego"
	"runtime"
	"unsafe"
)

type Input struct {
	client, port  uint32
	disposePort   func(uint32) int32
	disposeClient func(uint32) int32
	callback      uintptr
}

func Open(emit func([]byte)) (*Input, error) {
	library, err := purego.Dlopen("/System/Library/Frameworks/CoreMIDI.framework/CoreMIDI", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	var count func() uint64
	var source func(uint64) uint32
	var createClient func(uintptr, uintptr, uintptr, *uint32) int32
	var createPort func(uint32, uintptr, uintptr, uintptr, *uint32) int32
	var connect func(uint32, uint32, uintptr) int32
	var newString func(uintptr, string, uint32) uintptr
	var release func(uintptr)
	purego.RegisterLibFunc(&count, library, "MIDIGetNumberOfSources")
	purego.RegisterLibFunc(&source, library, "MIDIGetSource")
	purego.RegisterLibFunc(&createClient, library, "MIDIClientCreate")
	purego.RegisterLibFunc(&createPort, library, "MIDIInputPortCreate")
	purego.RegisterLibFunc(&connect, library, "MIDIPortConnectSource")
	purego.RegisterLibFunc(&newString, cf, "CFStringCreateWithCString")
	purego.RegisterLibFunc(&release, cf, "CFRelease")
	if count() == 0 {
		return nil, fmt.Errorf("midi: no MIDI input sources are available")
	}
	input := &Input{}
	purego.RegisterLibFunc(&input.disposePort, library, "MIDIPortDispose")
	purego.RegisterLibFunc(&input.disposeClient, library, "MIDIClientDispose")
	input.callback = purego.NewCallback(func(list unsafe.Pointer, _, _ uintptr) {
		packets := *(*uint32)(list)
		at := unsafe.Add(list, 4)
		for n := uint32(0); n < packets && n < 1024; n++ {
			length := int(*(*uint16)(unsafe.Add(at, 8)))
			if length > 65535 {
				return
			}
			data := append([]byte(nil), unsafe.Slice((*byte)(unsafe.Add(at, 10)), length)...)
			emit(data)
			at = unsafe.Add(at, 10+length)
			if runtime.GOARCH == "arm64" {
				at = unsafe.Add(at, (-uintptr(at))&3)
			}
		}
	})
	name := newString(0, "Go MaxYMiser", 0x08000100)
	defer release(name)
	if status := createClient(name, 0, 0, &input.client); status != 0 {
		return nil, fmt.Errorf("midi: client creation failed: %d", status)
	}
	if status := createPort(input.client, name, input.callback, 0, &input.port); status != 0 {
		input.Close()
		return nil, fmt.Errorf("midi: port creation failed: %d", status)
	}
	for i := uint64(0); i < count(); i++ {
		if status := connect(input.port, source(i), 0); status != 0 {
			input.Close()
			return nil, fmt.Errorf("midi: connection failed: %d", status)
		}
	}
	return input, nil
}
func (i *Input) Close() {
	if i == nil {
		return
	}
	if i.port != 0 {
		i.disposePort(i.port)
		i.port = 0
	}
	if i.client != 0 {
		i.disposeClient(i.client)
		i.client = 0
	}
}

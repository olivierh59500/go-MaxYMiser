//go:build darwin

package midi

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

type Output struct {
	client, port, destination uint32
	disposePort               func(uint32) int32
	disposeClient             func(uint32) int32
	send                      func(uint32, uint32, unsafe.Pointer) int32
	initPacket                func(unsafe.Pointer) unsafe.Pointer
	addPacket                 func(unsafe.Pointer, uint64, unsafe.Pointer, uint64, uint64, unsafe.Pointer) unsafe.Pointer
	buffer                    [512]byte
}

func Destinations() ([]Destination, error) {
	library, err := purego.Dlopen("/System/Library/Frameworks/CoreMIDI.framework/CoreMIDI", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	var count func() uint64
	var destination func(uint64) uint32
	var property func(uint32, uintptr, *uintptr) int32
	var newString func(uintptr, string, uint32) uintptr
	var toString func(uintptr, unsafe.Pointer, int64, uint32) bool
	var release func(uintptr)
	purego.RegisterLibFunc(&count, library, "MIDIGetNumberOfDestinations")
	purego.RegisterLibFunc(&destination, library, "MIDIGetDestination")
	purego.RegisterLibFunc(&property, library, "MIDIObjectGetStringProperty")
	purego.RegisterLibFunc(&newString, cf, "CFStringCreateWithCString")
	purego.RegisterLibFunc(&toString, cf, "CFStringGetCString")
	purego.RegisterLibFunc(&release, cf, "CFRelease")
	nameKey := newString(0, "displayName", 0x08000100)
	defer release(nameKey)
	var out []Destination
	for i := uint64(0); i < count(); i++ {
		id := destination(i)
		name := fmt.Sprintf("MIDI destination %d", i+1)
		var text uintptr
		if property(id, nameKey, &text) == 0 && text != 0 {
			var buffer [512]byte
			if toString(text, unsafe.Pointer(&buffer[0]), 512, 0x08000100) {
				length := 0
				for length < len(buffer) && buffer[length] != 0 {
					length++
				}
				name = string(buffer[:length])
			}
			release(text)
		}
		out = append(out, Destination{ID: id, Name: name})
	}
	return out, nil
}

func OpenOutput(destination uint32) (*Output, error) {
	all, err := Destinations()
	if err != nil {
		return nil, err
	}
	found := false
	for _, item := range all {
		found = found || item.ID == destination
	}
	if !found {
		return nil, fmt.Errorf("midi: selected output destination is unavailable")
	}
	library, err := purego.Dlopen("/System/Library/Frameworks/CoreMIDI.framework/CoreMIDI", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, err
	}
	var createClient func(uintptr, uintptr, uintptr, *uint32) int32
	var createPort func(uint32, uintptr, *uint32) int32
	var newString func(uintptr, string, uint32) uintptr
	var release func(uintptr)
	purego.RegisterLibFunc(&createClient, library, "MIDIClientCreate")
	purego.RegisterLibFunc(&createPort, library, "MIDIOutputPortCreate")
	purego.RegisterLibFunc(&newString, cf, "CFStringCreateWithCString")
	purego.RegisterLibFunc(&release, cf, "CFRelease")
	out := &Output{destination: destination}
	purego.RegisterLibFunc(&out.disposePort, library, "MIDIPortDispose")
	purego.RegisterLibFunc(&out.disposeClient, library, "MIDIClientDispose")
	purego.RegisterLibFunc(&out.send, library, "MIDISend")
	purego.RegisterLibFunc(&out.initPacket, library, "MIDIPacketListInit")
	purego.RegisterLibFunc(&out.addPacket, library, "MIDIPacketListAdd")
	name := newString(0, "Go MaxYMiser output", 0x08000100)
	defer release(name)
	if status := createClient(name, 0, 0, &out.client); status != 0 {
		return nil, fmt.Errorf("midi: output client creation failed: %d", status)
	}
	if status := createPort(out.client, name, &out.port); status != 0 {
		out.Close()
		return nil, fmt.Errorf("midi: output port creation failed: %d", status)
	}
	return out, nil
}

func (o *Output) Send(data []byte) error {
	if o == nil || o.port == 0 || len(data) == 0 || len(data) > 256 {
		return fmt.Errorf("midi: invalid output packet")
	}
	pointer := unsafe.Pointer(&o.buffer[0])
	packet := o.initPacket(pointer)
	if o.addPacket(pointer, uint64(len(o.buffer)), packet, 0, uint64(len(data)), unsafe.Pointer(&data[0])) == nil {
		return fmt.Errorf("midi: output packet did not fit")
	}
	status := o.send(o.port, o.destination, pointer)
	runtime.KeepAlive(data)
	if status != 0 {
		return fmt.Errorf("midi: send failed: %d", status)
	}
	return nil
}

func (o *Output) Close() {
	if o == nil {
		return
	}
	if o.port != 0 {
		o.disposePort(o.port)
		o.port = 0
	}
	if o.client != 0 {
		o.disposeClient(o.client)
		o.client = 0
	}
}

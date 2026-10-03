package sndh

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This is a foreign player made from 68000 instructions, with no MYM payload.
// It exercises the same init/exit/replay entry points used by real SNDH files.
func executableFixture(init, play, handler []byte) []byte {
	data := make([]byte, 512)
	for entry, target := range map[int]int{0: 96, 4: 176, 8: 192} {
		binary.BigEndian.PutUint16(data[entry:], 0x6000)
		binary.BigEndian.PutUint16(data[entry+2:], uint16(target-entry-2))
	}
	copy(data[12:], "SNDHTITLForeign CPU fixture\x00COMMTest\x00##02\x00!#01\x00TC50\x00HDNS")
	copy(data[96:], init)
	copy(data[176:], []byte{0x4e, 0x75})
	copy(data[192:], play)
	copy(data[320:], handler)
	return data
}

func byteWrite(value byte, address uint32) []byte {
	return []byte{0x13, 0xfc, 0, value, byte(address >> 24), byte(address >> 16), byte(address >> 8), byte(address)}
}
func longWrite(value, address uint32) []byte {
	return []byte{0x23, 0xfc, byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value), byte(address >> 24), byte(address >> 16), byte(address >> 8), byte(address)}
}
func registerWrite(register, value byte) []byte {
	return append(byteWrite(register, 0xff8800), byteWrite(value, 0xff8802)...)
}
func toneRoutine() []byte {
	var code []byte
	for _, pair := range [][2]byte{{0, 100}, {1, 0}, {7, 0x3e}, {8, 15}} {
		code = append(code, registerWrite(pair[0], pair[1])...)
	}
	return append(code, 0x4e, 0x75)
}

func TestRendererExecutesForeignInitAndProducesAudio(t *testing.T) {
	init := append(byteWrite(8, 0xff8800), []byte{0x13, 0xc0, 0, 0xff, 0x88, 2, 0x4e, 0x75}...)
	file, err := Parse(executableFixture(init, toneRoutine(), nil))
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(file, 2, 44100)
	if err != nil {
		t.Fatal(err)
	}
	if r.Registers()[8] != 2 {
		t.Fatalf("init D0 did not receive subtune: %d", r.Registers()[8])
	}
	buffer := make([]byte, 4410*4)
	n, err := r.Read(buffer)
	if err != nil || n != len(buffer) {
		t.Fatalf("Read = %d, %v", n, err)
	}
	if r.PositionSamples() != 4410 || r.Registers()[0] != 100 {
		t.Fatal("replay did not advance real chip state")
	}
	audible := false
	for at := 0; at < n; at += 4 {
		left, right := binary.LittleEndian.Uint16(buffer[at:]), binary.LittleEndian.Uint16(buffer[at+2:])
		if left != right {
			t.Fatal("mono PSG output must be duplicated into stereo")
		}
		audible = audible || left != 0
	}
	if !audible || r.HasEffects() {
		t.Fatal("ordinary foreign tone was not rendered correctly")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Read(buffer); err != io.EOF {
		t.Fatalf("closed renderer: %v", err)
	}
}

func TestRendererRunsTimerIRQAndRetainsEffects(t *testing.T) {
	init := longWrite(uploadAddress+320, 0x134)
	for _, pair := range []struct {
		value   byte
		address uint32
	}{{0x20, 0xfffa07}, {0x20, 0xfffa13}, {16, 0xfffa1f}, {1, 0xfffa19}} {
		init = append(init, byteWrite(pair.value, pair.address)...)
	}
	init = append(init, 0x4e, 0x75)
	handler := append(registerWrite(8, 9), 0x4e, 0x73)
	file, err := Parse(executableFixture(init, toneRoutine(), handler))
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(file, 1, 44100)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Read(make([]byte, 400)); err != nil {
		t.Fatal(err)
	}
	if !r.HasEffects() || r.Registers()[8] != 9 {
		t.Fatal("MFP interrupt did not execute its YM write")
	}
}

func TestRendererExecutesDMAPlayer(t *testing.T) {
	start, end := uint32(uploadAddress+400), uint32(uploadAddress+404)
	var init []byte
	for _, pair := range []struct {
		value   byte
		address uint32
	}{
		{byte(start >> 16), 0xff8903}, {byte(start >> 8), 0xff8905}, {byte(start), 0xff8907},
		{byte(end >> 16), 0xff890f}, {byte(end >> 8), 0xff8911}, {byte(end), 0xff8913},
		{0x82, 0xff8921}, {3, 0xff8901},
	} {
		init = append(init, byteWrite(pair.value, pair.address)...)
	}
	init = append(init, 0x4e, 0x75)
	data := executableFixture(init, []byte{0x4e, 0x75}, nil)
	copy(data[400:], []byte{100, 156, 80, 176})
	file, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(file, 1, 44100)
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 441*4)
	if _, err := r.Read(buffer); err != nil {
		t.Fatal(err)
	}
	low, high := int16(32767), int16(-32768)
	for at := 0; at < len(buffer); at += 4 {
		value := int16(binary.LittleEndian.Uint16(buffer[at:]))
		low = min(low, value)
		high = max(high, value)
	}
	if low > -4000 || high < 4000 || !r.HasEffects() {
		t.Fatalf("DMA audio range %d–%d effects %t", low, high, r.HasEffects())
	}
}

func TestRendererEmulatesGEMDOSMallocAndSupexec(t *testing.T) {
	// Push malloc(16), then store its real return register into guest memory.
	init := []byte{0x2f, 0x3c, 0, 0, 0, 16, 0x3f, 0x3c, 0, 0x48, 0x4e, 0x41, 0x5c, 0x8f, 0x23, 0xc0, 0, 0, 0x10, 0}
	// Push Supexec(callback), clean its arguments, and return from init.
	callback := uint32(uploadAddress + 320)
	init = append(init, 0x2f, 0x3c, byte(callback>>24), byte(callback>>16), byte(callback>>8), byte(callback), 0x3f, 0x3c, 0, 38, 0x4e, 0x4e, 0x5c, 0x8f, 0x4e, 0x75)
	handler := append(byteWrite(0x5a, 0x1004), 0x4e, 0x75)
	file, err := Parse(executableFixture(init, toneRoutine(), handler))
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(file, 1, 44100)
	if err != nil {
		t.Fatal(err)
	}
	if r.machine.Read32(0x1000) != ramSize-0x100000 || r.machine.Read8(0x1004) != 0x5a {
		t.Fatal("GEMDOS/Supexec changed the wrong stack or return register")
	}
}

func TestMachineBoundsEndlessExecution(t *testing.T) {
	m, err := newMachine(executableFixture([]byte{0x60, 0xfe}, toneRoutine(), nil), 44100)
	if err != nil {
		t.Fatal(err)
	}
	err = m.call(uploadAddress, 1, false, 128)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("endless player returned %v", err)
	}
}

func TestRendererPropagatesReplayFailure(t *testing.T) {
	// JMP to an unmapped address cannot silently become an empty recording.
	play := []byte{0x4e, 0xf9, 0, 0x50, 0, 0}
	file, err := Parse(executableFixture([]byte{0x4e, 0x75}, play, nil))
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(file, 1, 44100)
	if err != nil {
		t.Fatal(err)
	}
	if n, err := r.Read(make([]byte, 40)); n != 0 || err == nil {
		t.Fatalf("fault = %d, %v", n, err)
	}
	if _, err := r.Read(make([]byte, 40)); err == nil {
		t.Fatal("fault disappeared on next read")
	}
}

// Set SNDH_CORPUS to run private archive smoke checks without committing songs.
func TestRendererPrivateCorpus(t *testing.T) {
	root := os.Getenv("SNDH_CORPUS")
	if root == "" {
		t.Skip("SNDH_CORPUS is not set")
	}
	for _, name := range []string{"Mad_Max/Last_Ninja.sndh", "Mad_Max/Demos/Best_In_Galaxy/Commando.sndh", "Schuh_Paul/Cagney_and_Lacey.sndh", "Jarre_Jean_Michel/Captain_Blood.sndh", "Jess/Middle_Earth_Theme_(digit).sndh", "Griffon/DMA/STe_Folies.sndh", "Jason/Jason_Mix-STe_only.sndh", "Tao/TSD_STe/Airwaves.sndh", "Unknown_Composer/Digit_Tracker/InStyle.sndh", "Unknown_Composer/Digit_Tracker/Nice_One.sndh", "Unknown_Composer/Digit_Tracker/Work.sndh"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil {
				t.Fatal(err)
			}
			file, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			r, err := NewRenderer(file, file.Metadata.DefaultSubtune, 44100)
			if err != nil {
				t.Fatal(err)
			}
			buffer := make([]byte, 44100*4)
			if _, err := r.Read(buffer); err != nil {
				t.Fatal(err)
			}
			audible := false
			for _, b := range buffer {
				audible = audible || b != 0
			}
			if !audible {
				t.Fatal("no audio in first second")
			}
		})
	}
}

func TestRendererPrivateBrokenWrappersReturnErrors(t *testing.T) {
	root := os.Getenv("SNDH_CORPUS")
	if root == "" {
		t.Skip("SNDH_CORPUS is not set")
	}
	for _, name := range []string{"Kelly_Dave/Last_Ninja_2.sndh", "Zerkman/Ah_Que_Coucou.sndh"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil {
				t.Fatal(err)
			}
			file, err := Parse(data)
			if err != nil {
				t.Fatal(err)
			}
			r, err := NewRenderer(file, file.Metadata.DefaultSubtune, 44100)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := r.Read(make([]byte, 1024*4)); err == nil {
				t.Fatal("broken replay entry became a successful recording")
			}
		})
	}
}

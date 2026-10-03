package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func audibleFixture() []byte {
	data := make([]byte, 16)
	copy(data[12:], "SNDH")
	data = append(data, "TITLAudit fixture\x00##02\x00!#02\x00TC50\x00FLAG~y\x00HDNS"...)
	if len(data)%2 != 0 {
		data = append(data, 0)
	}
	init := len(data)
	// MOVE.B immediate into the YM latch/data ports produces a square wave.
	for _, pair := range [][2]byte{{0, 80}, {1, 0}, {7, 0x3e}, {8, 15}} {
		data = append(data, 0x13, 0xfc, 0, pair[0], 0, 0xff, 0x88, 0)
		data = append(data, 0x13, 0xfc, 0, pair[1], 0, 0xff, 0x88, 2)
	}
	data = append(data, 0x4e, 0x75)
	exit := len(data)
	data = append(data, 0x4e, 0x75)
	play := len(data)
	data = append(data, 0x4e, 0x75)
	for i, target := range []int{init, exit, play} {
		at := i * 4
		binary.BigEndian.PutUint16(data[at:], 0x6000)
		binary.BigEndian.PutUint16(data[at+2:], uint16(target-at-2))
	}
	return data
}

func readAudit(t *testing.T, path string) report {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var r report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAuditKeepsInputFailuresSeparateFromExecutedSubtunes(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "tone.SNDH"), audibleFixture(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "broken.sndh"), []byte("not a music executable"), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "report.json")
	err := run(context.Background(), []string{"-directory", directory, "-all-subtunes", "-seconds", "0.005", "-workers", "2", "-output", output})
	if err == nil {
		t.Fatal("input failure should produce a nonzero command result")
	}
	r := readAudit(t, output)
	c := r.Counts
	if c.FilesDiscovered != 2 || c.FilesMetadataParsed != 1 || c.FilesInitPassed != 1 || c.FilesReplayPassed != 1 || c.FilesErrors != 1 || c.SubtunesAttempted != 2 || c.SubtunesReplayPassed != 2 || c.SubtunesNonzeroAudio != 2 || c.SubtunesVariableAudio != 2 {
		t.Fatalf("audit stages were conflated: %+v", c)
	}
	if r.Files[0].Stage != "metadata" || r.Files[0].Error == "" || len(r.Files[0].Subtunes) != 0 {
		t.Fatalf("failed input: %+v", r.Files[0])
	}
	if !reflect.DeepEqual(r.Files[1].Selected, []int{1, 2}) {
		t.Fatalf("selected subtunes: %+v", r.Files[1])
	}
	for _, song := range r.Files[1].Subtunes {
		if song.Samples != 240 || song.Peak <= 0 || song.Minimum >= song.Maximum || song.RMS <= 0 || !song.ClosePassed || song.Error != "" || song.CloseError != "" {
			t.Fatalf("executed audio evidence: %+v", song)
		}
	}
}

func TestAuditDefaultSubtuneAndInputPreservation(t *testing.T) {
	input := filepath.Join(t.TempDir(), "tone.sndh")
	data := audibleFixture()
	if err := os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "report.json")
	if err := run(context.Background(), []string{"-input", input, "-seconds", "0.001", "-workers", "99", "-output", output}); err != nil {
		t.Fatal(err)
	}
	r := readAudit(t, output)
	if r.Workers != 8 || r.Counts.SubtunesAttempted != 1 || !reflect.DeepEqual(r.Files[0].Selected, []int{2}) {
		t.Fatalf("default selection and worker cap: %+v", r)
	}
	if err := run(context.Background(), []string{"-input", input, "-output", input}); err == nil {
		t.Fatal("report would overwrite an input music file")
	}
	got, err := os.ReadFile(input)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("input music was modified")
	}
}

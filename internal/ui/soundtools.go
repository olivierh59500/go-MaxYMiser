package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/olivierh59500/go-MaxYMiser/internal/edit"
	"github.com/olivierh59500/go-MaxYMiser/internal/model"
	"github.com/olivierh59500/go-MaxYMiser/internal/native"
	"github.com/olivierh59500/go-MaxYMiser/internal/replay"
)

func (a *App) drawSequenceTools(dst *ebiten.Image, e *replay.Engine) {
	a.text(dst, "SEQUENCE GENERATOR", 42, 260, 14, fg)
	for i, shape := range []edit.Shape{edit.Ramp, edit.Triangle, edit.Sine, edit.Square} {
		a.btn(dst, string(shape), 42+i*154, 292, 140, 34, "gen-shape:"+string(shape), a.generatorShape == shape)
	}
	a.btn(dst, "Signed words", 690, 292, 188, 34, "gen-signed", a.generatorSigned)
	for i, item := range []struct{ label, value, action string }{
		{"Minimum", a.generatorLow, "low"}, {"Maximum", a.generatorHigh, "high"}, {"Cycles", a.generatorCycles, "cycles"},
	} {
		x := 42 + i*256
		a.text(dst, item.label, float64(x), 350, 12, dim)
		a.btn(dst, item.value, x+92, 342, 146, 34, "gen-field:"+item.action, false)
	}
	a.btn(dst, "Generate", 862, 342, 168, 34, "gen-apply", false)
	a.btn(dst, "Modify range", 1044, 342, 186, 34, "gen-modify", false)
	a.text(dst, "Endpoints use hexadecimal words; cycles use decimal. Ramp holds its last value; waves loop.", 42, 401, 12, dim)
	length := max(1, min(63, int(e.Project.Bank.Sequences[a.sequence].Length)))
	low, high, cycles, err := a.generatorValues()
	if err == nil {
		sequence, err := edit.GenerateSequence(a.generatorShape, length, low, high, cycles, a.generatorSigned)
		if err == nil {
			rect(dst, 42, 439, 1192, 91, bg)
			value := func(word uint16) float32 {
				v := int(word)
				if a.generatorSigned {
					v = int(int16(word))
				}
				return 516 - float32(v-min(low, high))*65/float32(max(1, max(low, high)-min(low, high)))
			}
			for i := 0; i+1 < length; i++ {
				vector.StrokeLine(dst, 54+float32(i)*1168/float32(max(1, length-1)), value(sequence.Values[i]), 54+float32(i+1)*1168/float32(max(1, length-1)), value(sequence.Values[i+1]), 2, accent, false)
			}
		}
	} else {
		a.text(dst, err.Error(), 42, 454, 12, purple)
	}
	a.text(dst, fmt.Sprintf("MORPH · source %02X", a.sequence), 42, 565, 14, fg)
	a.btn(dst, "Destination "+a.morphDestination, 310, 557, 204, 34, "gen-field:destination", false)
	a.btn(dst, "Morph between", 530, 557, 198, 34, "gen-morph", false)
	a.btn(dst, "Copy to destination", 742, 557, 246, 34, "gen-copy", false)
	a.text(dst, "Morph fills the intervening sequence IDs. Endpoints keep their data; lengths and repeat must match.", 42, 621, 12, dim)
	a.text(dst, "Ctrl+Z undoes generation, morphing, copying or clearing.", 42, 647, 12, dim)
}

func parseSequenceWord(text string, signed bool) (int, error) {
	if strings.HasPrefix(text, "-") {
		if !signed {
			return 0, fmt.Errorf("enable signed words for negative endpoints")
		}
		value, err := strconv.ParseInt(text, 16, 16)
		return int(value), err
	}
	value, err := strconv.ParseUint(text, 16, 16)
	if signed {
		return int(int16(value)), err
	}
	return int(value), err
}

func (a *App) generatorValues() (int, int, float64, error) {
	low, err := parseSequenceWord(a.generatorLow, a.generatorSigned)
	if err != nil {
		return 0, 0, 0, err
	}
	high, err := parseSequenceWord(a.generatorHigh, a.generatorSigned)
	if err != nil {
		return 0, 0, 0, err
	}
	cycles, err := strconv.ParseFloat(a.generatorCycles, 64)
	return low, high, cycles, err
}

func (a *App) soundAction(name string) bool {
	if a.modal != "" {
		return false
	}
	if strings.HasPrefix(name, "gen-shape:") {
		a.generatorShape = edit.Shape(strings.TrimPrefix(name, "gen-shape:"))
		return true
	}
	if strings.HasPrefix(name, "gen-field:") {
		field := strings.TrimPrefix(name, "gen-field:")
		a.modal = "Generator " + field
		a.entry = map[string]string{"low": a.generatorLow, "high": a.generatorHigh, "cycles": a.generatorCycles, "destination": a.morphDestination}[field]
		return true
	}
	if strings.HasPrefix(name, "sample-gain:") {
		gain, _ := strconv.ParseFloat(strings.TrimPrefix(name, "sample-gain:"), 64)
		a.editSample(func(sample *model.Sample) error { return edit.AmplifySample(sample, gain) })
		return true
	}
	switch name {
	case "instrument-load":
		a.beginFileBrowser("Load instrument (.myi)", "", false)
	case "instrument-save":
		a.beginFileBrowser("Save instrument (.myi)", fmt.Sprintf("instrument-%02X.myi", a.instrument+1), true)
	case "instrument-copy":
		a.modal, a.entry = "Copy instrument (destination 01–20)", fmt.Sprintf("%02X", min(32, a.instrument+2))
	case "seq-tools":
		a.sequenceTools = !a.sequenceTools
	case "gen-signed":
		a.generatorSigned = !a.generatorSigned
	case "gen-modify":
		e, _ := a.synth.Snapshot()
		a.modal, a.entry = "Modify sequence (first,last,offset,scale)", fmt.Sprintf("0,%d,0,1", max(0, int(e.Project.Bank.Sequences[a.sequence].Length)-1))
	case "gen-apply":
		low, high, cycles, err := a.generatorValues()
		e, _ := a.synth.Snapshot()
		sequence := model.Sequence{}
		if err == nil {
			sequence, err = edit.GenerateSequence(a.generatorShape, max(1, int(e.Project.Bank.Sequences[a.sequence].Length)), low, high, cycles, a.generatorSigned)
		}
		if err != nil {
			a.status = err.Error()
			return true
		}
		a.remember()
		a.synth.Edit(func(e *replay.Engine) {
			e.Project.Bank.Sequences[a.sequence] = sequence
			e.Project.Bank.SequenceCount = max(e.Project.Bank.SequenceCount, a.sequence+1)
		})
		a.dirty, a.status = true, "Sequence generated"
	case "gen-morph", "gen-copy":
		destination, err := strconv.ParseUint(a.morphDestination, 16, 8)
		if err != nil {
			a.status = "Enter a hexadecimal destination sequence ID"
			return true
		}
		e, _ := a.synth.Snapshot()
		bank := e.Project.Bank
		if name == "gen-morph" {
			err = edit.MorphSequences(&bank, a.sequence, int(destination), a.generatorSigned)
		} else {
			bank.Sequences[destination] = bank.Sequences[a.sequence]
			bank.SequenceCount = max(bank.SequenceCount, int(destination)+1)
		}
		if err != nil {
			a.status = err.Error()
			return true
		}
		a.remember()
		a.synth.Edit(func(e *replay.Engine) { e.Project.Bank = bank })
		a.dirty, a.status = true, "Sequence bank updated"
	case "seq-clear":
		a.remember()
		a.synth.Edit(func(e *replay.Engine) { e.Project.Bank.Sequences[a.sequence] = model.Sequence{Length: 1} })
		a.dirty = true
	case "sample-tune":
		a.modal, a.entry = "Tune sample (semitones)", "0.125"
	case "sample-trim":
		e, _ := a.synth.Snapshot()
		a.modal, a.entry = "Trim sample (start,length)", fmt.Sprintf("0,%d", len(e.Project.Bank.Samples[a.sample].PCM))
	case "sample-sign":
		a.editSample(func(sample *model.Sample) error { edit.ToggleSampleSign(sample); return nil })
	case "sample-ymise":
		a.editSample(func(sample *model.Sample) error { edit.YMiseSample(sample); return nil })
	case "sample-save":
		a.beginFileBrowser("Save signed PCM sample", fmt.Sprintf("sample-%d.pcm", a.sample+1), true)
	case "sample-preview":
		if _, ok := a.synth.Reference(); ok {
			a.synth.SelectReference(false)
		}
		a.synth.Edit(func(e *replay.Engine) { e.Stop(); e.TriggerSample(0, 60, byte(a.sample+1)) })
	default:
		return false
	}
	return true
}

func (a *App) editSample(operation func(*model.Sample) error) {
	e, _ := a.synth.Snapshot()
	sample := e.Project.Bank.Samples[a.sample]
	if err := operation(&sample); err != nil {
		a.status = err.Error()
		return
	}
	a.remember()
	a.synth.Edit(func(e *replay.Engine) { e.Project.Bank.Samples[a.sample] = sample })
	a.dirty, a.status = true, "Sample edited"
}

func (a *App) soundModal(modal, entry string) bool {
	if strings.HasPrefix(modal, "Generator ") {
		switch strings.TrimPrefix(modal, "Generator ") {
		case "low":
			a.generatorLow = entry
		case "high":
			a.generatorHigh = entry
		case "cycles":
			a.generatorCycles = entry
		case "destination":
			a.morphDestination = entry
		}
		return true
	}
	switch modal {
	case "Modify sequence (first,last,offset,scale)":
		parts := strings.Split(entry, ",")
		if len(parts) != 4 {
			a.status = "Enter decimal first,last,offset,scale"
			return true
		}
		first, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		last, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		offset, e3 := strconv.Atoi(strings.TrimSpace(parts[2]))
		scale, e4 := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
			a.status = "Enter decimal first,last,offset,scale"
			return true
		}
		e, _ := a.synth.Snapshot()
		sequence := e.Project.Bank.Sequences[a.sequence]
		if err := edit.ModifySequence(&sequence, first, last, offset, scale, a.generatorSigned); err != nil {
			a.status = err.Error()
		} else {
			a.remember()
			a.synth.Edit(func(e *replay.Engine) { e.Project.Bank.Sequences[a.sequence] = sequence })
			a.dirty, a.status = true, "Sequence range modified"
		}
	case "Copy instrument (destination 01–20)":
		to, err := strconv.ParseUint(entry, 16, 8)
		if err != nil || to < 1 || to > 32 {
			a.status = "Enter an instrument ID from 01 to 20"
			return true
		}
		a.remember()
		a.synth.Edit(func(e *replay.Engine) { edit.CopyInstrument(&e.Project.Bank, a.instrument, int(to)-1) })
		a.dirty, a.status = true, "Instrument copied with shared sequence links"
	case "Load instrument (.myi)":
		raw, err := os.ReadFile(entry)
		var file native.InstrumentFile
		if err == nil {
			file, err = native.DecodeInstrument(raw)
		}
		e, _ := a.synth.Snapshot()
		bank := e.Project.Bank
		if err == nil {
			err = native.ImportInstrument(&bank, a.instrument, file)
		}
		if err != nil {
			a.status = err.Error()
		} else {
			a.remember()
			a.synth.Edit(func(e *replay.Engine) { e.Project.Bank = bank; e.Stop() })
			a.dirty, a.status = true, "Native instrument loaded with independent sequence slots"
		}
	case "Save instrument (.myi)":
		e, _ := a.synth.Snapshot()
		file, err := native.ExportInstrument(&e.Project.Bank, a.instrument)
		var raw []byte
		if err == nil {
			raw, err = native.EncodeInstrument(file)
		}
		if err == nil && a.icePacking {
			raw, err = native.PackICE(raw)
		}
		if err == nil {
			var output *os.File
			output, err = os.OpenFile(entry, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err == nil {
				_, err = output.Write(raw)
				closeErr := output.Close()
				if err == nil {
					err = closeErr
				}
			}
		}
		if err != nil {
			a.status = err.Error()
		} else {
			a.status = "Native MYI3 instrument saved"
		}
	case "Tune sample (semitones)":
		semitones, err := strconv.ParseFloat(entry, 64)
		if err != nil {
			a.status = "Enter a decimal tuning in semitones"
		} else {
			a.editSample(func(sample *model.Sample) error { return edit.TuneSample(sample, semitones) })
		}
	case "Trim sample (start,length)":
		parts := strings.Split(entry, ",")
		if len(parts) != 2 {
			a.status = "Enter start,length in decimal bytes"
			return true
		}
		start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		length, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil {
			a.status = "Enter start,length in decimal bytes"
		} else {
			a.editSample(func(sample *model.Sample) error { return edit.TrimSample(sample, start, length) })
		}
	case "Save signed PCM sample":
		e, _ := a.synth.Snapshot()
		file, err := os.OpenFile(entry, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err == nil {
			_, err = file.Write(e.Project.Bank.Samples[a.sample].PCM)
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
		}
		if err != nil {
			a.status = err.Error()
		} else {
			a.status = "Signed PCM sample saved"
		}
	default:
		return false
	}
	return true
}

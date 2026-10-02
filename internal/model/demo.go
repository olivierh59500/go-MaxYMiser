package model

// Demo is an original composition for trying the editor without external data.
func Demo() *Project {
	p := New()
	p.Title = "First signal"
	p.Author = "Malakh Software"
	p.Song.Length = 4
	p.Song.Patterns = make([]Pattern, 8)
	p.Song.Orders[0] = [4]byte{0, 4, 6, EmptyPattern}
	p.Song.Orders[1] = [4]byte{1, 5, 6, EmptyPattern}
	p.Song.Orders[2] = [4]byte{2, 4, 7, EmptyPattern}
	p.Song.Orders[3] = [4]byte{3, 5, 7, EmptyPattern}
	p.Bank.Instruments[1] = p.Bank.Instruments[0]
	p.Bank.Instruments[1].SetName("Chord pulse")
	p.Bank.Instruments[1][49] = 3
	p.Bank.Sequences[3] = Sequence{Values: [63]uint16{0, 4, 7}, Length: 3, Repeat: 0}
	p.Bank.Instruments[2] = p.Bank.Instruments[0]
	p.Bank.Instruments[2].SetName("Noise drum")
	p.Bank.Instruments[2][48] = 4
	p.Bank.Instruments[2][51] = 5
	p.Bank.Instruments[2][52] = 6
	p.Bank.Sequences[4] = Sequence{Values: [63]uint16{15, 12, 9, 6, 3, 0}, Length: 6, Repeat: 5}
	p.Bank.Sequences[5] = Sequence{Values: [63]uint16{0x1000}, Length: 1}
	p.Bank.Sequences[6] = Sequence{Values: [63]uint16{8, 12, 16, 20, 24}, Length: 5, Repeat: 4}
	melodies := [][8]byte{{60, 64, 67, 72, 71, 67, 64, 62}, {57, 60, 64, 69, 67, 64, 60, 59}, {53, 57, 60, 65, 64, 60, 57, 55}, {55, 59, 62, 67, 65, 62, 59, 57}}
	for pat, melody := range melodies {
		for step, note := range melody {
			p.Song.Patterns[pat][step*8] = Cell{Note: note, Instrument: 1}
		}
	}
	for pat := 4; pat < 6; pat++ {
		for row := 0; row < 64; row += 16 {
			note := byte(36)
			if pat == 5 {
				note = 33
			}
			p.Song.Patterns[pat][row] = Cell{Note: note, Instrument: 2}
		}
	}
	for pat := 6; pat < 8; pat++ {
		for row := 0; row < 64; row += 8 {
			p.Song.Patterns[pat][row] = Cell{Note: 60, Instrument: 3}
		}
	}
	return p
}

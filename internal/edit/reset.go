package edit

import "github.com/olivierh59500/go-MaxYMiser/internal/model"

// ClearSong removes notes and arrangement content while retaining the current
// sound bank, metadata, native versions and personal playback/editing settings.
func ClearSong(project *model.Project) {
	fresh := model.New().Song
	fresh.Version = project.Song.Version
	fresh.State = project.Song.State
	clear(fresh.State[:4])
	for _, start := range []int{4, 52} {
		copy(fresh.State[start:start+4], fresh.Orders[0][:])
	}
	project.Song = fresh
}

// ClearBank removes instruments, sequence definitions and sample payloads.
// Notes and their numeric sound references remain editable in the same song.
func ClearBank(project *model.Project) {
	bank := model.VoiceBank{Version: project.Bank.Version, SampleVersion: project.Bank.SampleVersion, SequenceCount: 1}
	for index := range bank.Sequences {
		bank.Sequences[index].Length = 1
	}
	project.Bank = bank
}

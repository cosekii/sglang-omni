package main

import "github.com/sgl-project/sglang-omni/Voxt/internal/settings"

type dictionaryNotesViewState struct {
	Enabled  bool `json:"enabled"`
	NotesOff bool `json:"notes_off"`
}

type dictionaryNotesStateInput struct {
	Settings settings.Settings
}

func buildDictionaryNotesViewState(input dictionaryNotesStateInput) dictionaryNotesViewState {
	return dictionaryNotesViewState{Enabled: input.Settings.Dictionary.Enabled, NotesOff: input.Settings.Dictionary.NotesOff}
}

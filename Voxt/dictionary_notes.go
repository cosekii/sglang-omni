package main

// The Personal dictionary view: Markdown notes, autosave, and find. Feature
// pages show where the notes and glossary land in their whole-prompt preview.

import (
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/sgl-project/sglang-omni/Voxt/internal/i18n"
)

func notesPlaceholder() string {
	return i18n.T("| Standard spelling | Correction mapping | Condition |\n|---|---|---|\n| 101; I-80 | `one oh one` → 101 | Bay Area highways |\n| Roth | `wrath` → Roth | retirement accounts |\n| NVDA | — | tickers, all caps |")
}

func dictionaryNotesView(c *ui.Context, state *dictionaryNotesViewState, actions dictionaryActions, local *dictionaryLocalUI) {
	t := c.Theme()
	ui.Scroll(c.Key("dict-notes-scroll")).Grow(1).MinHeight(0).Children(func() {
		ui.Column(c).Padding(px(t, sp2), px(t, sp6), px(t, sp6)).Gap(px(t, sp2)).Children(func() {
			caption(c, i18n.T("Write your names, terms and numbers once, in Markdown: for example a table of the standard spelling, what speech recognition writes instead, and when it applies. Transcribe, Translate and Rewrite add it to the language model's prompt. Speech recognition does not get it."))
			off := !state.Enabled
			formRow(c, "dict-notes-on-row", i18n.T("Add to the language model prompt"), "", func() {
				on := !state.NotesOff
				if ui.Switch(c.Key("dict-notes-on"), &on).Label(i18n.T("Add to the language model prompt")).Disabled(off).Changed() {
					actions.setDictionaryNotesOff(!on)
				}
			})
			if off {
				notice(c, "dict-notes-off", noticeInfo, i18n.T("The dictionary is off, so the personal dictionary is not sent."), "", nil, nil)
			}
			matches := findFolded(local.notes.text, local.notesQuery)
			moved := local.notesFind.advance(len(matches))
			ui.Row(c).Gap(px(t, sp2)).AlignItems(ui.Center).Children(func() {
				local.notesFind.field(c, "dict-notes-find", &local.notesQuery, i18n.T("Find in the personal dictionary"), px(t, 320)).Label(i18n.T("Search personal dictionary"))
				local.notesFind.count(c, local.notesQuery, len(matches))
			})
			input := ui.TextArea(c.Key("dict-notes"), &local.notes.text).Label(i18n.T("Personal dictionary in Markdown")).Placeholder(notesPlaceholder()).Lines(8, math.MaxInt32)
			if len(matches) > 0 {
				ranges := make([]ui.TextRange, len(matches))
				for index, match := range matches {
					ranges[index] = ui.TextRange{Start: match[0], End: match[1], Color: tok(t).Danger, Weight: 700}
					if index == local.notesFind.cur-1 {
						ranges[index].Weight = 900
					}
				}
				input.TextRanges(ranges...)
			}
			if moved {
				input.SetTextSelection(matches[local.notesFind.cur-1][0], matches[local.notesFind.cur-1][1])
			}
			*state = actions.autosaveDictionaryNotes(input.Changed(), input.Focused())
			caption(c, i18n.T("Each feature's page shows its own whole request, with the variables filled in: open Your rules, then What the language model receives."))
		})
	})
}

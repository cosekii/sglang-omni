package main

import (
	"bytes"
	"testing"

	"github.com/sgl-project/sglang-omni/Voxt/internal/deliver"
	"github.com/sgl-project/sglang-omni/Voxt/internal/dict"
	"github.com/sgl-project/sglang-omni/Voxt/internal/i18n"
	"github.com/sgl-project/sglang-omni/Voxt/internal/session"
	"github.com/sgl-project/sglang-omni/Voxt/internal/settings"
	"github.com/sgl-project/sglang-omni/Voxt/internal/store"
)

func testDictionaryHighlights(text string) []dict.Span {
	return dict.HighlightSpans(text, []store.Term{
		{Term: "Voxt", Enabled: true},
		{Term: "SGLang", Aliases: []string{"S G Long"}, Enabled: true},
	})
}

// TestTranscribeDictionaryHighlights: matched terms keep the whole result
// readable and selectable, with a visible tint in the Try preview and the
// answer card.
func TestTranscribeDictionaryHighlights(t *testing.T) {
	const result = "Voxt uses SGLang."
	t.Run("Try preview", func(t *testing.T) {
		a, tt := trTester(t)
		s := trySnapshot(a, session.Done)
		s.Raw, s.Result = "voxt uses s g long", result
		a.sessionChanged(s)
		frames(tt)
		before := bytes.Clone(tt.Image().Pix)
		s.Rev++
		s.Highlights = testDictionaryHighlights(result)
		a.sessionChanged(s)
		frames(tt)
		if bytes.Equal(before, tt.Image().Pix) {
			t.Error("the matched terms have no visible highlight")
		}
		if !hasExact(tt, result) {
			t.Errorf("the whole result is not readable: %q", tt.Texts())
		}
	})
	t.Run("answer card", func(t *testing.T) {
		a, _, _ := newTestApp(t)
		shortScreen(t, a)
		s := session.Snapshot{ID: "dictionary-card", Kind: store.KindTranscribe, State: session.Done, Card: true,
			Delivery: deliver.ReasonCard, Result: result}
		snap(a, s)
		before := renderOverlay(t, a)
		s.Highlights = testDictionaryHighlights(result)
		a.openAnswer(s)
		a.overlaySync()
		after := renderOverlay(t, a)
		if bytes.Equal(before.Pix, after.Pix) {
			t.Error("the matched terms have no visible highlight")
		}
	})
}

func TestTryBuilderDictionaryHighlights(t *testing.T) {
	const result = "  Voxt uses SGLang.  "
	state := buildTryCardViewState(i18n.NewTranslator("en"), tryCardStateInput{Mine: true,
		Snapshot: session.Snapshot{ID: "try-dictionary", Kind: store.KindTranscribe, State: session.Done,
			Result: result, Highlights: testDictionaryHighlights(result)}})
	if state.Ended == nil || len(state.Ended.Highlights) != 2 || state.Ended.Highlights[0] != (dictionaryHighlightSpan{Start: 0, End: 4}) || state.Ended.Highlights[1] != (dictionaryHighlightSpan{Start: 10, End: 16}) {
		t.Fatalf("Try result spans: %+v", state.Ended)
	}
}

func TestOnboardingTryBuilderDictionaryHighlights(t *testing.T) {
	const result = "  Voxt uses SGLang.  "
	const sessionID = "onboarding-dictionary"
	state := buildOnboardingTry(i18n.NewTranslator("en"), onboardingViewStateInput{
		Step: obTranscribe, Settings: settings.Defaults(), Tries: map[int]string{obTranscribe: sessionID},
		Session: session.Snapshot{ID: sessionID, Kind: store.KindTranscribe, State: session.Done,
			Result: result, Highlights: testDictionaryHighlights(result)},
	})
	if len(state.Card.Highlights) != 2 || state.Card.Highlights[0] != (dictionaryHighlightSpan{Start: 0, End: 4}) || state.Card.Highlights[1] != (dictionaryHighlightSpan{Start: 10, End: 16}) {
		t.Fatalf("onboarding Try result spans: %+v", state.Card.Highlights)
	}
}

// TestRedoDropsOldHighlights: a redo streams a new draft over the card;
// the old answer's spans must not mark it (or slice outside it).
func TestRedoDropsOldHighlights(t *testing.T) {
	const long = "Voxt uses SGLang, and Voxt keeps it fast."
	a, _, _ := newTestApp(t)
	shortScreen(t, a)
	s := session.Snapshot{ID: "t1", Kind: store.KindTranscribe, State: session.Done, Card: true,
		Delivery: deliver.ReasonCard, Result: long}
	snap(a, s)
	s.Highlights = testDictionaryHighlights(long)
	a.openAnswer(s)
	a.overlaySync()
	a.answer.redo = "t2"
	snap(a, session.Snapshot{ID: "t2", Kind: store.KindTranscribe, State: session.Composing, Card: true, Draft: "短一些。"})
	card := a.cachedCard()
	if text := cardTextForTest(card); text != "短一些。" || card.LiveCount != 1 || len(card.Live[0].Highlights) != 0 {
		t.Errorf("the redo keeps the old spans over %q: %+v", text, card.Live[0])
	}
	a.overlaySync()
	tt := cardTester(t, a)
	frames(tt)
	if !hasExact(tt, "短一些。") {
		t.Errorf("the draft is not the card's text: %q", firstTexts(tt))
	}
}

// TestDictionaryHighlightOutOfRange: a span from an older, longer text
// is clamped to this one instead of slicing outside it.
func TestDictionaryHighlightOutOfRange(t *testing.T) {
	a, _, _ := newTestApp(t)
	shortScreen(t, a)
	s := session.Snapshot{ID: "t1", Kind: store.KindTranscribe, State: session.Done, Card: true,
		Delivery: deliver.ReasonCard, Result: "short"}
	snap(a, s)
	s.Highlights = []dict.Span{{Start: 2, End: 40}}
	a.openAnswer(s)
	a.overlaySync()
	renderOverlay(t, a)
}

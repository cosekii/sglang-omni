package session

import (
	"errors"
	"testing"
	"time"

	"github.com/sgl-project/sglang-omni/Voxt/internal/llm"
	"github.com/sgl-project/sglang-omni/Voxt/internal/settings"
	"github.com/sgl-project/sglang-omni/Voxt/internal/store"
)

// TestUsedUpPlanIsNotFallenBackFrom: a signed-in plan out of usage ends
// the session; the fallback, another service, is not asked behind the
// user's back.
func TestUsedUpPlanIsNotFallenBackFrom(t *testing.T) {
	spent := &llm.Error{Code: llm.CodeUsageLimit}
	main, fallback := &fakeLLM{err: spent}, &fakeLLM{text: "早安"}
	h := newHarness(t, &fakeFile{text: "x"}, main, func(c *Config, _ *settings.Settings) { c.Select = &fakeSelector{text: "Good morning"} })
	h.fallback = fallback
	if _, err := h.m.Start(StartRequest{Kind: store.KindTranslate, Target: slack}); err != nil {
		t.Fatal(err)
	}
	end := h.rec.wait(t)
	if end.State != Failed || end.Problem.Code != ProblemLLM || llm.Code(end.Problem.Err) != llm.CodeUsageLimit {
		t.Errorf("end %+v", end)
	}
	if n := len(fallback.requests()); n != 0 {
		t.Errorf("%d requests to the fallback", n)
	}
}

// TestTranslateFallback: when the language model fails, Translate asks
// its fallback model once, which then answers the rest of the session
// (the strict retry too); the snapshot and the entry name the model that
// answered last.
func TestTranslateFallback(t *testing.T) {
	boom := errors.New("503 overloaded")
	for _, tc := range []struct {
		name             string
		main, fallback   *fakeLLM
		state            State
		result, model    string
		mainN, fallbackN int
	}{
		{"the model answers", &fakeLLM{text: "早上好"}, &fakeLLM{text: "早安"}, Done, "早上好", "deepseek", 1, 0},
		{"the model fails", &fakeLLM{err: boom}, &fakeLLM{text: "早安"}, Done, "早安", "ollama", 1, 1},
		{"the fallback is asked again strictly", &fakeLLM{err: boom}, &fakeLLM{texts: []string{"Good morning.", "早安"}}, Done, "早安", "ollama", 1, 2},
		{"both fail", &fakeLLM{err: boom}, &fakeLLM{err: boom}, Failed, "", "ollama", 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, &fakeFile{text: "x"}, tc.main, func(c *Config, _ *settings.Settings) { c.Select = &fakeSelector{text: "Good morning"} })
			h.fallback = tc.fallback
			if _, err := h.m.Start(StartRequest{Kind: store.KindTranslate, Target: slack}); err != nil {
				t.Fatal(err)
			}
			end := h.rec.wait(t)
			if end.State != tc.state || end.Result != tc.result || end.LLM.ID != tc.model {
				t.Errorf("end %s %q by %q, want %s %q by %q (problem %+v)", end.State, end.Result, end.LLM.ID, tc.state, tc.result, tc.model, end.Problem)
			}
			if n, m := len(tc.main.requests()), len(tc.fallback.requests()); n != tc.mainN || m != tc.fallbackN {
				t.Errorf("%d requests to the model, %d to the fallback; want %d, %d", n, m, tc.mainN, tc.fallbackN)
			}
			if es := h.hist.all(); len(es) != 1 || es[0].LLMProvider != tc.model || es[0].LLMModel != end.LLM.Model {
				t.Errorf("entries %+v", es)
			}
		})
	}
}

// TestRewriteFallbackDropsTheFirstModelsTitle: the first model streamed
// a title before it failed. Once the fallback takes over, no snapshot
// and no history entry shows that title next to the fallback's name:
// the swap starts the answer over, as asking again does.
func TestRewriteFallbackDropsTheFirstModelsTitle(t *testing.T) {
	boom := errors.New("503 overloaded")
	main, fallback := &fakeLLM{hang: true, hangOn: 1, hangDelta: `{"title":"First model's title","content":"Half`}, &fakeLLM{err: boom}
	h := newHarness(t, &fakeFile{text: " hello aura link "}, main, func(c *Config, _ *settings.Settings) {
		c.Select = &fakeSelector{}
		c.WriteTimeout = time.Second
	})
	h.fallback = fallback
	id, err := h.m.Start(StartRequest{Kind: store.KindRewrite, Target: slack})
	if err != nil {
		t.Fatal(err)
	}
	h.speak(id, 3)
	h.rec.waitFor(t, "the first model's title", func(s Snapshot) bool { return s.State == Composing && s.Title == "First model's title" })
	end := h.rec.wait(t)
	if end.State != Failed || end.Problem == nil || end.Problem.Code != ProblemLLM || end.LLM.ID != "ollama" {
		t.Errorf("end %+v", end)
	}
	for _, s := range h.rec.all() {
		if s.LLM.ID == "ollama" && s.Title == "First model's title" {
			t.Errorf("the fallback's snapshot keeps the first model's title: %+v", s)
		}
	}
	if es := h.hist.all(); len(es) != 1 || es[0].LLMProvider != "ollama" || es[0].Title != "" {
		t.Errorf("entries %+v", es)
	}
}

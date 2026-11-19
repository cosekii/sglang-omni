package compose

import (
	"strings"
	"testing"
)

func TestParseAnswer(t *testing.T) {
	for _, tc := range []struct {
		text string
		want Answer
		ok   bool
	}{
		{`{"title":"Update","content":"Ships Friday."}`, Answer{"Update", "Ships Friday."}, true},
		{"```json\n{\"title\": \" **Update** \", \"content\": \"Line 1\\nLine 2\"}\n```", Answer{"Update", "Line 1\nLine 2"}, true},
		{`Sure! {"title":"T","content":"x {y} z"} hope it helps`, Answer{"T", "x {y} z"}, true},
		{`{"title":"T","content":"  "}`, Answer{Title: "T"}, true},
		{`{"title":"T","text":"x"}`, Answer{}, false},
		{`{"title":"T"`, Answer{}, false},
		{"just text", Answer{}, false},
		{`{bad} {"content":"second"}`, Answer{Content: "second"}, true},
	} {
		got, ok := ParseAnswer(tc.text)
		if ok != tc.ok || got != tc.want {
			t.Errorf("ParseAnswer(%q) = %+v %v", tc.text, got, ok)
		}
	}
}

func TestPartialAnswer(t *testing.T) {
	full := `{"title": "周五发布", "content": "大家好：\n- VoxtNew \"0.1\" 周五发布 \ud83d\ude80 \u4f60"}`
	var last Answer
	for i := 1; i <= len(full); i++ {
		a := PartialAnswer(full[:i])
		if !strings.HasPrefix(full, full[:i]) || len(a.Content) < len(last.Content) && !strings.HasPrefix(last.Content, a.Content) {
			t.Fatalf("at %d: %+v after %+v", i, a, last)
		}
		if strings.ContainsRune(a.Content, '\ufffd') {
			t.Fatalf("at %d: a broken rune in %q", i, a.Content)
		}
		last = a
	}
	if last.Title != "周五发布" || last.Content != "大家好：\n- VoxtNew \"0.1\" 周五发布 🚀 你" {
		t.Errorf("final %+v", last)
	}
	for text, want := range map[string]Answer{
		"":                               {},
		"Plain text answer":              {Content: "Plain text answer"},
		"```":                            {},
		"```json\n{\"title\":\"T\",\"co": {Title: "T"},
		`{"content":"a\`:                 {Content: "a"},
		`{"content": "x", "title": "`:    {Content: "x"},
	} {
		if got := PartialAnswer(text); got != want {
			t.Errorf("PartialAnswer(%q) = %+v", text, got)
		}
	}
}

// TestStripControlAnswer: ParseAnswer and PartialAnswer strip a model's
// internal control blocks too (#4085), a partial one's unfinished tail
// included.
func TestStripControlAnswer(t *testing.T) {
	if got := StripControl("a<|x|>b<|tool_call_be"); got != "a<|x|>b" {
		t.Errorf("StripControl %q", got)
	}
	// A bare word is no control tag: F#'s pipe keeps its operand.
	if got := StripControl("let y = f <|x"); got != "let y = f <|x" {
		t.Errorf("StripControl %q", got)
	}
	if got := StripControl("no blocks"); got != "no blocks" {
		t.Errorf("StripControl %q", got)
	}
	a, ok := ParseAnswer(`{"title":"T<|x|>","content":"Ships Friday.<|tool_call_begin|>c<|tool_call_end|>"}`)
	if !ok || a.Title != "T<|x|>" || a.Content != "Ships Friday." {
		t.Errorf("ParseAnswer %+v %v", a, ok)
	}
	if got := PartialAnswer(`{"title":"T","content":"Ships Friday.<|tool_call_be`); got.Content != "Ships Friday." {
		t.Errorf("PartialAnswer %+v", got)
	}
	if got := PartialAnswer("plain text <|tool_calls_section_be"); got.Content != "plain text" {
		t.Errorf("PartialAnswer %+v", got)
	}
	a, ok = ParseAnswer(`{"title":"T<|tool_call_end|>","content":"Ships Friday.<|tool_call_end|>"}`)
	if !ok || a.Title != "T" || a.Content != "Ships Friday." {
		t.Errorf("ParseAnswer with a close marker %+v %v", a, ok)
	}
	if got := PartialAnswer(`{"title":"T","content":"Ships Friday.<|tool_call_end|>"}`); got.Content != "Ships Friday." {
		t.Errorf("PartialAnswer with a close marker %+v", got)
	}
	if got := PartialAnswer("plain text<|tool_call_end|>"); got.Content != "plain text" {
		t.Errorf("PartialAnswer with a close marker %+v", got)
	}
}

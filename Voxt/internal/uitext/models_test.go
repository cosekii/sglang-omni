package uitext

import (
	"strings"
	"testing"

	"github.com/sgl-project/sglang-omni/Voxt/internal/asr"
	"github.com/sgl-project/sglang-omni/Voxt/internal/i18n"
	"github.com/sgl-project/sglang-omni/Voxt/internal/llm"
	"github.com/sgl-project/sglang-omni/Voxt/internal/local"
)

func TestResponsesOnlyText(t *testing.T) {
	msg, more := ModelErrorText(&llm.Error{Code: llm.CodeResponsesOnly, Status: 404})
	if !strings.Contains(msg, "API format") || !strings.Contains(more, llm.CodeResponsesOnly) {
		t.Errorf("%q, %q", msg, more)
	}
}

func TestModelErrorTextForTranslator(t *testing.T) {
	translator := i18n.NewTranslator("zh-Hans")
	msg, more := ModelErrorTextFor(translator, local.ErrNotInstalled)
	if msg != translator.T("The model is not installed.") || more != local.ErrNotInstalled.Error() {
		t.Fatalf("translator error text: %q, %q", msg, more)
	}
}

func TestModelErrorTextForSettingsGuidance(t *testing.T) {
	translator := i18n.NewTranslator("en")
	for _, test := range []struct {
		err  error
		want string
	}{
		{err: &asr.Error{Kind: asr.KindConfig}, want: "Settings → Speech models"},
		{err: &llm.Error{Code: llm.CodeSignIn}, want: "Settings → Language models"},
		{err: &llm.Error{Code: llm.CodeNoKey}, want: "Settings → Language models"},
		{err: &llm.Error{Code: llm.CodeAddress}, want: "Settings → Language models"},
	} {
		message, _ := ModelErrorTextFor(translator, test.err)
		if !strings.Contains(message, test.want) {
			t.Errorf("%v: %q", test.err, message)
		}
	}
}

func TestPlanErrorTexts(t *testing.T) {
	for code, want := range map[string]string{
		llm.CodeSignIn:      "Sign in again",
		llm.CodeUsageLimit:  "usage limit",
		llm.CodeNotEligible: "cannot use Codex",
	} {
		if msg, _ := ModelErrorText(&llm.Error{Code: code}); !strings.Contains(msg, want) {
			t.Errorf("%s: %q", code, msg)
		}
	}
}

package main

// The Speech models section in Settings (/settings/models; /models before):
// local and remote speech models, retired local language models, shared-family
// status, downloads and deletion. Controllers own the model library and runtime
// work; this file only composes view state.

import (
	"github.com/egoist/mygo/ui"
	"github.com/sgl-project/sglang-omni/Voxt/internal/i18n"
)

// modelsPath is the Speech models section's path in Settings.
const modelsPath = "/settings/models"

// modelsPageViews contains the adjacent areas' views, not model behavior.
// The fixture demo leaves them empty.
type modelsPageViews struct {
	onboardingBackBar func()
	remoteASRCards    func()
	remoteDialog      func()
}

// modelsSettingsBody is the reusable Speech models body for Settings.
func modelsSettingsBody(c *ui.Context, state *modelsViewState, actions modelsActions, local *modelsLocalUI, external modelsPageViews) {
	ui.Row(c).Justify(ui.End).Children(func() {
		if ghostButton(c.Key("models-network"), i18n.T("Network…")).Tooltip(i18n.T("How model downloads reach Hugging Face")).Clicked() {
			actions.openNetwork()
		}
	})
	modelsSections(c, state, actions, local, external)
}

func modelsSections(c *ui.Context, state *modelsViewState, actions modelsActions, local *modelsLocalUI, external modelsPageViews) {
	if external.onboardingBackBar != nil {
		external.onboardingBackBar()
	}
	sectionTitle(c, i18n.T("On this Mac"))
	caption(c, i18n.T("Free, and your audio stays on this Mac."))
	modelCards(c, state.ASR, actions)
	if state.Dictation != nil {
		modelCard(c, *state.Dictation, actions)
	}
	if external.remoteASRCards != nil {
		external.remoteASRCards()
	}
	speechContextSection(c, state.SpeechContext, actions, &local.speechContext)
	if state.ShowLLM {
		sectionTitleTip(c, i18n.T("Language models no longer used"), i18n.T("Voxt no longer uses these. Delete them to free up space."))
		modelCards(c, state.LLM, actions)
	}
	hiddenModelCards(c, state.Hidden, &local.modelHiddenOpen, actions)
	modelsAdvanced(c, &state.Advanced, actions, local)
}

func modelsOverlays(c *ui.Context, state *modelsViewState, actions modelsActions, local *modelsLocalUI, external modelsPageViews) {
	modelDeleteDialogs(c, &state.Delete, actions, local)
	networkDialog(c, &state.Network, actions, local)
	if external.remoteDialog != nil {
		external.remoteDialog()
	}
}

// modelsPage adds the standalone page chrome used by the fixture demo.
func modelsPage(c *ui.Context, state *modelsViewState, actions modelsActions, local *modelsLocalUI, external modelsPageViews) {
	t := c.Theme()
	columnPageHeader(c, i18n.T("Speech models"), func() {
		if ghostButton(c.Key("models-network"), i18n.T("Network…")).Tooltip(i18n.T("How model downloads reach Hugging Face")).Clicked() {
			actions.openNetwork()
		}
	})
	ui.Scroll(c.Key("models-scroll")).Grow(1).MinHeight(0).Children(func() {
		pageColumn(c).Gap(px(t, sp3)).Children(func() {
			modelsSections(c, state, actions, local, external)
		})
	})
	modelsOverlays(c, state, actions, local, external)
}

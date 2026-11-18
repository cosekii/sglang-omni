package main

// Dictionary profile views. The controller owns profile persistence and the
// active profile; this file only renders serializable view state and local UI.

import (
	"github.com/egoist/mygo/ui"
	"github.com/sgl-project/sglang-omni/Voxt/internal/i18n"
	"github.com/sgl-project/sglang-omni/Voxt/internal/icons"
)

type dictionaryProfileViewState struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	RawName string `json:"raw_name"`
	Active  bool   `json:"active"`
}

type dictionaryProfileEditViewState struct {
	Open      bool   `json:"open"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Error     string `json:"error"`
	ErrorCode string `json:"error_code"`
	Saving    bool   `json:"saving"`
}

type dictionaryProfileDeleteViewState struct {
	Open bool   `json:"open"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type dictionaryProfilesViewState struct {
	Rows   []dictionaryProfileViewState     `json:"rows"`
	Edit   dictionaryProfileEditViewState   `json:"edit"`
	Delete dictionaryProfileDeleteViewState `json:"delete"`
}

type dictionaryProfileLocalUI struct {
	editOpen   bool
	editID     string
	editName   string
	editErr    string
	editSaving bool
	deleteOpen bool
	deleteID   string
}

func dictionaryProfileBar(c *ui.Context, state *dictionaryProfilesViewState, actions dictionaryActions, local *dictionaryLocalUI) {
	t := c.Theme()
	if len(state.Rows) == 0 {
		return
	}
	active := dictionaryProfileViewState{}
	selected, names := 0, make([]string, len(state.Rows))
	for index, profile := range state.Rows {
		names[index] = profile.Name
		if profile.Active {
			active, selected = profile, index
		}
	}
	profileUI := &local.profiles
	ui.Row(c).Gap(px(t, sp2)).AlignItems(ui.Center).Wrap().Children(func() {
		caption(c, i18n.T("Active profile"))
		if segmentedFit(c.Key("dict-profile"), &selected, names...).Label(i18n.T("Active dictionary profile")).Changed() {
			actions.switchDictionaryProfile(state.Rows[selected].ID)
		}
		if ghostIconButton(c.Key("dict-profile-new"), icons.Add, i18n.T("New profile")).Clicked() {
			profileUI.editOpen, profileUI.editID, profileUI.editName, profileUI.editErr, profileUI.editSaving = true, "", "", "", false
		}
		moreMenu(c.Key("dict-profile-more"), i18n.T("Profile actions"), func(menu *ui.Menu) {
			if menu.Item(i18n.T("Rename “%s”…", active.Name)).Disabled(active.ID == "").Chosen() {
				profileUI.editOpen, profileUI.editID, profileUI.editName, profileUI.editErr, profileUI.editSaving = true, active.ID, active.RawName, "", false
			}
			if menu.Item(i18n.T("Delete “%s”…", active.Name)).Disabled(len(state.Rows) <= 1).Chosen() {
				profileUI.deleteOpen, profileUI.deleteID = true, active.ID
			}
		})
	})
}

func dictionaryProfileDialogs(c *ui.Context, state *dictionaryProfilesViewState, actions dictionaryActions, local *dictionaryLocalUI) {
	profileUI := &local.profiles
	if profileUI.editOpen {
		title := i18n.T("New profile")
		if profileUI.editID != "" {
			title = i18n.T("Rename profile")
		}
		dialogFrame(c, "dict-profile-edit", &profileUI.editOpen, title, "", 420, func() {
			fieldInfo(c, i18n.T("Name"), i18n.T("Use different dictionary entries for different situations."), func() {
				input := ui.TextInput(c.Key("dict-profile-name"), &profileUI.editName).Label(i18n.T("Profile name")).Placeholder(i18n.T("Work")).AutoFocus()
				if input.Changed() {
					profileUI.editErr = ""
				}
				if input.Submitted() {
					actions.saveDictionaryProfile()
				}
			})
			if profileUI.editErr != "" {
				warnLine(c, profileUI.editErr)
			}
		}, func() {
			ui.Spacer(c)
			if ui.Button(c.Key("dict-profile-cancel"), i18n.T("Cancel")).Clicked() {
				profileUI.editOpen = false
			}
			if primaryButton(c.Key("dict-profile-save"), i18n.T("Save")).Disabled(profileUI.editSaving).Clicked() {
				actions.saveDictionaryProfile()
			}
		})
	}
	if profileUI.deleteOpen {
		dialogFrame(c, "dict-profile-delete", &profileUI.deleteOpen, i18n.T("Delete “%s”?", state.Delete.Name), "", 420, func() {
			ui.Text(c, i18n.T("Its dictionary entries will be deleted. This cannot be undone."))
		}, func() {
			ui.Spacer(c)
			if ui.Button(c.Key("dict-profile-delete-cancel"), i18n.T("Cancel")).AutoFocus().Clicked() {
				profileUI.deleteOpen = false
			}
			if recordButton(c.Key("dict-profile-delete-ok"), i18n.T("Delete")).Clicked() {
				profileUI.deleteOpen = false
				actions.deleteDictionaryProfile()
			}
		})
	}
}

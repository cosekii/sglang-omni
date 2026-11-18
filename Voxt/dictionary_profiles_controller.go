package main

// Dictionary profiles are named sets of terms. Repository reads and writes use
// the active profile, so sessions, imports and learning follow the page and the
// voxtnew://dictionary/profile deep link.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sgl-project/sglang-omni/Voxt/internal/core"
	"github.com/sgl-project/sglang-omni/Voxt/internal/i18n"
	"github.com/sgl-project/sglang-omni/Voxt/internal/store"
)

type dictionaryProfileState struct {
	list  []store.DictionaryProfile
	reads int
}

type dictionaryProfileEditStateInput struct {
	Open   bool
	ID     string
	Name   string
	Error  string
	Err    error
	Saving bool
}

type dictionaryProfilesStateInput struct {
	Profiles   []store.DictionaryProfile
	Edit       dictionaryProfileEditStateInput
	DeleteOpen bool
	DeleteID   string
}

func dictionaryProfileNameFor(translator *i18n.Translator, profile store.DictionaryProfile) string {
	if profile.ID == "default" {
		return translator.T("Default")
	}
	return profile.Name
}

func profileName(profile store.DictionaryProfile) string {
	return dictionaryProfileNameFor(i18n.CurrentTranslator(), profile)
}

func buildDictionaryProfilesViewState(translator *i18n.Translator, input dictionaryProfilesStateInput) dictionaryProfilesViewState {
	state := dictionaryProfilesViewState{
		Edit: dictionaryProfileEditViewState{
			Open: input.Edit.Open, ID: input.Edit.ID, Name: input.Edit.Name, Error: input.Edit.Error, Saving: input.Edit.Saving,
		},
		Delete: dictionaryProfileDeleteViewState{Open: input.DeleteOpen, ID: input.DeleteID},
	}
	for _, profile := range input.Profiles {
		name := dictionaryProfileNameFor(translator, profile)
		state.Rows = append(state.Rows, dictionaryProfileViewState{ID: profile.ID, Name: name, RawName: profile.Name, Active: profile.Active})
		if input.DeleteOpen && profile.ID == input.DeleteID {
			state.Delete.Name = name
		}
	}
	if input.Edit.Err != nil {
		state.Edit.Error = profileErrorTextFor(translator, input.Edit.Err)
	}
	switch {
	case errors.Is(input.Edit.Err, core.ErrExists):
		state.Edit.ErrorCode = "exists"
	case errors.Is(input.Edit.Err, core.ErrInvalid):
		state.Edit.ErrorCode = "invalid"
	case input.Edit.Err != nil:
		state.Edit.ErrorCode = "generic"
	}
	return state
}

func dictionaryProfilesFingerprint(profiles []store.DictionaryProfile) string {
	var fingerprint strings.Builder
	for _, profile := range profiles {
		fmt.Fprintf(&fingerprint, "%s\x00%s\x00%t|", profile.ID, profile.Name, profile.Active)
	}
	return fingerprint.String()
}

func (a *app) activeDictionaryProfile() store.DictionaryProfile {
	for _, profile := range a.dictProfiles.list {
		if profile.Active {
			return profile
		}
	}
	return store.DictionaryProfile{}
}

func (a *app) loadDictionaryProfiles() {
	a.dictProfiles.reads++
	read, repo := a.dictProfiles.reads, a.lib.store.Dictionary()
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		profiles, err := repo.Profiles(ctx)
		a.post(func() {
			if read != a.dictProfiles.reads {
				return
			}
			if err != nil {
				a.toast(i18n.T("Could not read the dictionary profiles: %v", err))
				return
			}
			a.dictProfiles.list = profiles
		})
	})
}

func (a *app) markDictionaryProfileActive(id string) {
	for index := range a.dictProfiles.list {
		a.dictProfiles.list[index].Active = a.dictProfiles.list[index].ID == id
	}
}

func (a *app) switchDictionaryProfile(id string) {
	repo := a.lib.store.Dictionary()
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := repo.ActivateProfile(ctx, id)
		a.post(func() {
			if err != nil {
				a.toast(i18n.T("Could not switch the dictionary profile: %v", err))
				return
			}
			a.markDictionaryProfileActive(id)
			a.loadDict()
		})
	})
}

// switchDictionaryProfileByName accepts both the stored name and the translated
// built-in Default name without changing navigation.
func (a *app) switchDictionaryProfileByName(name string) {
	repo := a.lib.store.Dictionary()
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		profiles, err := repo.Profiles(ctx)
		a.post(func() {
			if err != nil {
				a.toast(i18n.T("Could not switch the dictionary profile: %v", err))
				return
			}
			name = strings.TrimSpace(name)
			for _, profile := range profiles {
				if profile.Name == name || profileName(profile) == name {
					a.switchDictionaryProfile(profile.ID)
					return
				}
			}
			a.toast(i18n.T("No dictionary profile named “%s”", name))
		})
	})
}

func profileErrorTextFor(translator *i18n.Translator, err error) string {
	switch {
	case errors.Is(err, core.ErrExists):
		return translator.T("A profile with this name already exists.")
	case errors.Is(err, core.ErrInvalid):
		return translator.T("Type a profile name.")
	default:
		return translator.T("Could not save the profile: %v", err)
	}
}

func profileErrorText(err error) string {
	return profileErrorTextFor(i18n.CurrentTranslator(), err)
}

func (a *app) saveDictionaryProfile() {
	profileUI := &a.dictUI.profiles
	name := strings.TrimSpace(profileUI.editName)
	if name == "" {
		profileUI.editErr = i18n.T("Type a profile name.")
		return
	}
	profileUI.editSaving = true
	id, repo := profileUI.editID, a.lib.store.Dictionary()
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		if id == "" {
			err = repo.CreateProfile(ctx, store.DictionaryProfile{ID: core.NewID(time.Now()), Name: name})
		} else {
			err = repo.RenameProfile(ctx, id, name)
		}
		a.post(func() {
			profileUI := &a.dictUI.profiles
			profileUI.editSaving = false
			if err != nil {
				profileUI.editErr = profileErrorText(err)
				return
			}
			profileUI.editOpen = false
			a.loadDictionaryProfiles()
		})
	})
}

func (a *app) deleteDictionaryProfile() {
	id, repo := a.dictUI.profiles.deleteID, a.lib.store.Dictionary()
	a.async(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := repo.DeleteProfile(ctx, id)
		a.post(func() {
			if err != nil {
				a.toast(i18n.T("Could not delete the profile: %v", err))
				return
			}
			a.loadDictionaryProfiles()
			a.loadDict()
		})
	})
}

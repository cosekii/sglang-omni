package main

import (
	"github.com/egoist/mygo"
	"github.com/sgl-project/sglang-omni/Voxt/internal/settings"
)

func (a *app) overlaySize(mode overlayMode) (width, height int, exact bool) {
	if mode == ovAnswer {
		return a.cardSize()
	}
	return a.sessionOverlaySize(&a.ov.projection.view)
}

func (a *app) sessionOverlaySize(state *sessionOverlayViewState) (width, height int, exact bool) {
	overlay := a.prefs.Overlay
	if a.measure == nil {
		a.measure = a.ovText
	}
	model := sessionOverlayLayoutFor(state, sessionOverlayLayoutInput{
		Measure:  a.measure,
		WorkArea: a.ov.area,
		Overlay: sessionOverlayLayoutStyle{
			Position: overlay.Position, Opacity: overlay.Opacity, Radius: overlay.Radius, Margin: overlay.Margin,
			ShowTime: overlay.ShowTime, LiveLines: overlay.LiveLines,
		},
		Memory: sessionOverlayLayoutMemory{
			LiveID: a.ov.live.id, LiveWidth: a.ov.live.w, StreamHeight: a.ov.stream.h,
		},
		Measured:         a.ov.textW,
		MarkdownMeasured: a.ov.mdH,
		Reveal:           a.ov.reveal,
	})
	a.ov.live.id, a.ov.live.w = model.LiveID, model.LiveWidth
	a.ov.stream.h = model.StreamHeight
	a.ov.layout = model
	return model.Width, model.Height, model.Exact
}

func faceRadius(overlay settings.Overlay, mode overlayMode, height int) float32 {
	radius := float32(overlay.Radius)
	if mode != ovAnswer {
		radius = min(radius, float32(height)/2)
	}
	return radius
}

func (a *app) ovTextWidth(key ovSized) (float32, bool) {
	measured, ok := a.ovText(key)
	return measured.w, ok
}

func (a *app) ovText(key ovSized) (ovMeasured, bool) {
	if measured, ok := a.ov.textW[key]; ok {
		return measured, true
	}
	if a.measureHook != nil {
		width, height := a.measureHook(key)
		return ovMeasured{width, height}, true
	}
	width, height := guessText(key)
	return ovMeasured{width, height}, false
}

func mergeSessionOverlayMeasurements(destination map[ovSized]ovMeasured, measurements []sessionOverlayMeasurement) (map[ovSized]ovMeasured, bool) {
	if len(measurements) == 0 {
		return destination, false
	}
	changed := destination == nil || len(destination) > 256
	if changed {
		destination = map[ovSized]ovMeasured{}
	}
	for _, measurement := range measurements {
		measured := ovMeasured{measurement.Width, measurement.Height}
		if current, ok := destination[measurement.Key]; ok && current == measured {
			continue
		}
		destination[measurement.Key] = measured
		changed = true
	}
	return destination, changed
}

func (a *app) postSessionOverlayMeasurements(measurements []sessionOverlayMeasurement) {
	if len(measurements) == 0 {
		return
	}
	a.post(func() {
		var textMeasurements, markdownMeasurements []sessionOverlayMeasurement
		for _, measurement := range measurements {
			if measurement.Markdown {
				markdownMeasurements = append(markdownMeasurements, measurement)
			} else {
				textMeasurements = append(textMeasurements, measurement)
			}
		}
		var textChanged, markdownChanged bool
		a.ov.textW, textChanged = mergeSessionOverlayMeasurements(a.ov.textW, textMeasurements)
		a.ov.mdH, markdownChanged = mergeSessionOverlayMeasurements(a.ov.mdH, markdownMeasurements)
		if textChanged || markdownChanged {
			a.overlayRestyle()
		}
	})
}

var ovTestArea = mygo.Rectangle{X: 0, Y: 25, Width: 1440, Height: 875}

func (a *app) overlayArea() mygo.Rectangle {
	if !a.overlayNative() {
		return ovTestArea
	}
	return mygo.Screen.DisplayNearestPoint(mygo.Screen.CursorScreenPoint()).WorkArea
}

func (a *app) cardSize() (width, height int, exact bool) {
	return a.sessionOverlaySize(&a.ov.projection.view)
}

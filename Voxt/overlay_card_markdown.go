package main

// Markdown in the answer card (#4119): a Rewrite's body and conversation
// answers render as the model wrote them; user, Translate, and Transcribe
// text stays plain. Rendered heights use a separate measurement cache.

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/sgl-project/sglang-omni/Voxt/internal/overlay/layout"
)

// mdReveal shows a streaming answer as it comes, without pacing or fades:
// the card paces it itself (overlay_stream.go).
var mdReveal = markdown.Reveal{Unit: markdown.AtOnce, Fade: -1}

func sessionCardBlockUsesMarkdown(card *sessionCardViewState, block sessionCardBlock) bool {
	if block.Kind == sessionCardBubble {
		return !block.User
	}
	return card.Kind == "rewrite" && block.Kind == sessionCardBody && block.Text != ""
}

// cardMarkdown renders Stream while the model writes and View once it is done.
func cardMarkdown(context *ui.Context, key ovSized, color ui.Color, live bool, local sessionOverlayLocal, onMeasured func([]sessionOverlayMeasurement)) ui.Element {
	options := local.Overlay.mdCard
	if key.size == layout.BubbleTextSize {
		options = local.Overlay.mdBubble
	}
	if options == nil {
		options = &markdown.Options{}
		if key.size == layout.BubbleTextSize {
			local.Overlay.mdBubble = options
		} else {
			local.Overlay.mdCard = options
		}
	}
	if color == (ui.Color{}) {
		color = ui.RGBA(255, 255, 255, 0.92)
	}
	options.Theme = &markdown.Theme{
		FontSize:            key.size,
		Text:                color,
		Muted:               ui.RGBA(255, 255, 255, 0.6),
		Link:                tok(context.Theme()).Accent,
		QuoteBar:            ui.RGBA(255, 255, 255, 0.24),
		QuoteText:           ui.RGBA(255, 255, 255, 0.6),
		Rule:                ovCardRule,
		CodeBackground:      ui.RGBA(255, 255, 255, 0.1),
		CodeBlockBackground: ui.RGBA(255, 255, 255, 0.06),
		CodeBlockBorder:     ui.RGBA(255, 255, 255, 0.1),
		TableBorder:         ui.RGBA(255, 255, 255, 0.12),
		TableHeader:         ui.RGBA(255, 255, 255, 0.06),
	}
	options.Reveal = &mdReveal
	var element ui.Element
	if live {
		element = markdown.Stream(context, key.text, options)
	} else {
		element = markdown.View(context, key.text, options)
	}
	if live {
		element.Opacity(0.91)
	}
	return element.Draw(func(_ *ui.Painter, rectangle ui.Rect) {
		if onMeasured != nil {
			onMeasured([]sessionOverlayMeasurement{{Key: key, Width: rectangle.W, Height: rectangle.H, Markdown: true}})
		}
	})
}

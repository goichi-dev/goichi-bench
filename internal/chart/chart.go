// Package chart renders benchmark results as standalone SVG.
//
// The charts use emphasis rather than eight hues: one framework is the subject
// and carries the accent colour, every other framework is a neutral grey. The
// story is "how does goichi compare", not "which of six colours is which", and
// a reader never has to match a swatch to a legend — every bar is named on its
// own row and labelled with its own value.
package chart

import (
	"fmt"
	"strings"
)

// Colours are the light-mode palette; dark-mode equivalents are declared in the
// stylesheet embedded in every SVG.
const (
	surfaceLight   = "#fcfcfb"
	textPrimary    = "#0b0b0b"
	textSecondary  = "#52514e"
	textMuted      = "#898781"
	accentLight    = "#2a78d6"
	accentDark     = "#3987e5"
	neutralBar     = "#898781"
	surfaceDark    = "#1a1a19"
	textPrimaryDk  = "#ffffff"
	textSecondaryD = "#c3c2b7"
)

// Bar is one row of a panel.
type Bar struct {
	Label     string
	Value     float64
	Display   string // pre-formatted value; "n/a" when the value is missing
	Missing   bool
	Highlight bool
}

// Panel is one small multiple: a titled group of bars sharing a scale.
type Panel struct {
	Title string
	Note  string
	Bars  []Bar
}

// Options control the whole figure.
type Options struct {
	Title    string
	Subtitle string
	// Columns is how many panels sit side by side.
	Columns int
}

const (
	panelW    = 330
	labelW    = 78
	valueW    = 74
	rowH      = 24
	barH      = 12
	panelPadY = 34
	figPadX   = 24
	headerH   = 74
	footerH   = 30
)

// Render returns a complete standalone SVG document.
func Render(opts Options, panels []Panel) string {
	if opts.Columns <= 0 {
		opts.Columns = 3
	}
	cols := opts.Columns
	rows := (len(panels) + cols - 1) / cols

	maxBars := 0
	for _, p := range panels {
		if len(p.Bars) > maxBars {
			maxBars = len(p.Bars)
		}
	}
	panelH := panelPadY + maxBars*rowH + 12
	width := figPadX*2 + cols*panelW
	height := headerH + rows*panelH + footerH

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="%s">`,
		width, height, width, height, esc(opts.Title))
	b.WriteString(style())
	fmt.Fprintf(&b, `<rect width="%d" height="%d" class="gb-surface"/>`, width, height)

	fmt.Fprintf(&b, `<text x="%d" y="34" class="gb-title">%s</text>`, figPadX, esc(opts.Title))
	if opts.Subtitle != "" {
		fmt.Fprintf(&b, `<text x="%d" y="56" class="gb-subtitle">%s</text>`, figPadX, esc(opts.Subtitle))
	}

	for i, p := range panels {
		x := figPadX + (i%cols)*panelW
		y := headerH + (i/cols)*panelH
		renderPanel(&b, p, x, y)
	}

	fmt.Fprintf(&b, `<text x="%d" y="%d" class="gb-footnote">goichi is highlighted; every other framework is grey. Bars are labelled with their own value.</text>`,
		figPadX, height-12)
	b.WriteString(`</svg>`)
	return b.String()
}

func renderPanel(b *strings.Builder, p Panel, x, y int) {
	fmt.Fprintf(b, `<text x="%d" y="%d" class="gb-panel-title">%s</text>`, x, y+12, esc(p.Title))
	if p.Note != "" {
		fmt.Fprintf(b, `<text x="%d" y="%d" class="gb-panel-note">%s</text>`, x, y+27, esc(p.Note))
	}

	max := 0.0
	for _, bar := range p.Bars {
		if !bar.Missing && bar.Value > max {
			max = bar.Value
		}
	}
	if max <= 0 {
		max = 1
	}

	trackW := panelW - labelW - valueW - 16
	for i, bar := range p.Bars {
		rowY := y + panelPadY + i*rowH
		textY := rowY + barH - 2

		fmt.Fprintf(b, `<text x="%d" y="%d" class="gb-row-label">%s</text>`, x+labelW-8, textY, esc(bar.Label))

		if bar.Missing {
			fmt.Fprintf(b, `<text x="%d" y="%d" class="gb-row-missing">%s</text>`, x+labelW, textY, esc(bar.Display))
			continue
		}

		w := int(float64(trackW) * bar.Value / max)
		if w < 2 {
			w = 2
		}
		class := "gb-bar"
		if bar.Highlight {
			class = "gb-bar gb-accent"
		}
		b.WriteString(roundedBar(x+labelW, rowY, w, barH, class))
		fmt.Fprintf(b, `<text x="%d" y="%d" class="gb-row-value">%s</text>`, x+labelW+w+8, textY, esc(bar.Display))
	}
}

// roundedBar draws a bar with a 4px rounded data-end and a square baseline,
// falling back to a plain rectangle when the bar is too short to round.
func roundedBar(x, y, w, h int, class string) string {
	const r = 4
	if w <= r*2 {
		return fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" class="%s"/>`, x, y, w, h, class)
	}
	return fmt.Sprintf(
		`<path d="M%d %d H%d a%d %d 0 0 1 %d %d V%d a%d %d 0 0 1 %d %d H%d Z" class="%s"/>`,
		x, y, x+w-r, r, r, r, r, y+h-r, r, r, -r, r, x, class,
	)
}

func style() string {
	return `<style>
    .gb-surface { fill: ` + surfaceLight + `; }
    .gb-title { font: 600 17px system-ui, -apple-system, "Segoe UI", sans-serif; fill: ` + textPrimary + `; }
    .gb-subtitle { font: 400 12.5px system-ui, -apple-system, "Segoe UI", sans-serif; fill: ` + textSecondary + `; }
    .gb-panel-title { font: 600 12.5px system-ui, -apple-system, "Segoe UI", sans-serif; fill: ` + textPrimary + `; }
    .gb-panel-note { font: 400 11px system-ui, -apple-system, "Segoe UI", sans-serif; fill: ` + textMuted + `; }
    .gb-row-label { font: 400 11.5px system-ui, -apple-system, "Segoe UI", sans-serif; fill: ` + textSecondary + `; text-anchor: end; }
    .gb-row-value { font: 400 11.5px system-ui, -apple-system, "Segoe UI", sans-serif; fill: ` + textSecondary + `; font-variant-numeric: tabular-nums; }
    .gb-row-missing { font: 400 11.5px system-ui, -apple-system, "Segoe UI", sans-serif; fill: ` + textMuted + `; }
    .gb-footnote { font: 400 10.5px system-ui, -apple-system, "Segoe UI", sans-serif; fill: ` + textMuted + `; }
    .gb-bar { fill: ` + neutralBar + `; }
    .gb-bar.gb-accent { fill: ` + accentLight + `; }
    @media (prefers-color-scheme: dark) {
      .gb-surface { fill: ` + surfaceDark + `; }
      .gb-title { fill: ` + textPrimaryDk + `; }
      .gb-subtitle, .gb-row-label, .gb-row-value { fill: ` + textSecondaryD + `; }
      .gb-panel-title { fill: ` + textPrimaryDk + `; }
      .gb-bar.gb-accent { fill: ` + accentDark + `; }
    }
  </style>`
}

func esc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

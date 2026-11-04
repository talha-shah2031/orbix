// Package theme contains terminal styles shared by the TUI views.
package theme

import "github.com/charmbracelet/lipgloss"

type Palette struct {
	Accent             lipgloss.AdaptiveColor
	Green              lipgloss.AdaptiveColor
	Red                lipgloss.AdaptiveColor
	Muted              lipgloss.AdaptiveColor
	Panel              lipgloss.AdaptiveColor
	HeatPositive       lipgloss.AdaptiveColor
	HeatPositiveMid    lipgloss.AdaptiveColor
	HeatPositiveStrong lipgloss.AdaptiveColor
	HeatNegative       lipgloss.AdaptiveColor
	HeatNegativeMid    lipgloss.AdaptiveColor
	HeatNegativeStrong lipgloss.AdaptiveColor
	HeatNeutral        lipgloss.AdaptiveColor
	HeatText           lipgloss.AdaptiveColor
	HeatBorder         lipgloss.AdaptiveColor
}

var Dark = Palette{
	Accent:             lipgloss.AdaptiveColor{Light: "#6246EA", Dark: "#8B7CFF"},
	Green:              lipgloss.AdaptiveColor{Light: "#16803C", Dark: "#5CE08A"},
	Red:                lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#FF6B6B"},
	Muted:              lipgloss.AdaptiveColor{Light: "#667085", Dark: "#9299A6"},
	Panel:              lipgloss.AdaptiveColor{Light: "#F1F3F8", Dark: "#20232B"},
	HeatPositive:       lipgloss.AdaptiveColor{Light: "#137333", Dark: "#176B45"},
	HeatPositiveMid:    lipgloss.AdaptiveColor{Light: "#16803C", Dark: "#218653"},
	HeatPositiveStrong: lipgloss.AdaptiveColor{Light: "#0E5C2D", Dark: "#2A9C61"},
	HeatNegative:       lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#A92E39"},
	HeatNegativeMid:    lipgloss.AdaptiveColor{Light: "#9E2632", Dark: "#C13B48"},
	HeatNegativeStrong: lipgloss.AdaptiveColor{Light: "#7E1E2A", Dark: "#DB4A56"},
	HeatNeutral:        lipgloss.AdaptiveColor{Light: "#475467", Dark: "#424A57"},
	HeatText:           lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"},
	HeatBorder:         lipgloss.AdaptiveColor{Light: "#344054", Dark: "#171B22"},
}

var palettes = []Palette{
	Dark,
	{
		Accent:             lipgloss.AdaptiveColor{Light: "#4937B4", Dark: "#C4BAFF"},
		Green:              lipgloss.AdaptiveColor{Light: "#137333", Dark: "#247A3B"},
		Red:                lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#B3261E"},
		Muted:              lipgloss.AdaptiveColor{Light: "#4B5563", Dark: "#4B5563"},
		Panel:              lipgloss.AdaptiveColor{Light: "#EEF0F5", Dark: "#EEF0F5"},
		HeatPositive:       lipgloss.AdaptiveColor{Light: "#137333", Dark: "#137333"},
		HeatPositiveMid:    lipgloss.AdaptiveColor{Light: "#16803C", Dark: "#16803C"},
		HeatPositiveStrong: lipgloss.AdaptiveColor{Light: "#0E5C2D", Dark: "#0E5C2D"},
		HeatNegative:       lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#B3261E"},
		HeatNegativeMid:    lipgloss.AdaptiveColor{Light: "#9E2632", Dark: "#9E2632"},
		HeatNegativeStrong: lipgloss.AdaptiveColor{Light: "#7E1E2A", Dark: "#7E1E2A"},
		HeatNeutral:        lipgloss.AdaptiveColor{Light: "#475467", Dark: "#475467"},
		HeatText:           lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"},
		HeatBorder:         lipgloss.AdaptiveColor{Light: "#344054", Dark: "#344054"},
	},
	{
		Accent:             lipgloss.AdaptiveColor{Light: "#FFFF00", Dark: "#FFFF00"},
		Green:              lipgloss.AdaptiveColor{Light: "#00FF00", Dark: "#00FF00"},
		Red:                lipgloss.AdaptiveColor{Light: "#FF5555", Dark: "#FF5555"},
		Muted:              lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"},
		Panel:              lipgloss.AdaptiveColor{Light: "#000000", Dark: "#000000"},
		HeatPositive:       lipgloss.AdaptiveColor{Light: "#00A000", Dark: "#00A000"},
		HeatPositiveMid:    lipgloss.AdaptiveColor{Light: "#00CC00", Dark: "#00CC00"},
		HeatPositiveStrong: lipgloss.AdaptiveColor{Light: "#00FF00", Dark: "#00FF00"},
		HeatNegative:       lipgloss.AdaptiveColor{Light: "#FF5555", Dark: "#FF5555"},
		HeatNegativeMid:    lipgloss.AdaptiveColor{Light: "#FF3333", Dark: "#FF3333"},
		HeatNegativeStrong: lipgloss.AdaptiveColor{Light: "#FF0000", Dark: "#FF0000"},
		HeatNeutral:        lipgloss.AdaptiveColor{Light: "#666666", Dark: "#666666"},
		HeatText:           lipgloss.AdaptiveColor{Light: "#000000", Dark: "#000000"},
		HeatBorder:         lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"},
	},
	{
		Accent:             lipgloss.AdaptiveColor{Light: "#005A9C", Dark: "#79C7FF"},
		Green:              lipgloss.AdaptiveColor{Light: "#0067A5", Dark: "#61B5E8"},
		Red:                lipgloss.AdaptiveColor{Light: "#B54708", Dark: "#FFB05C"},
		Muted:              lipgloss.AdaptiveColor{Light: "#667085", Dark: "#9299A6"},
		Panel:              lipgloss.AdaptiveColor{Light: "#F1F3F8", Dark: "#20232B"},
		HeatPositive:       lipgloss.AdaptiveColor{Light: "#005A9C", Dark: "#005A9C"},
		HeatPositiveMid:    lipgloss.AdaptiveColor{Light: "#0077B6", Dark: "#0077B6"},
		HeatPositiveStrong: lipgloss.AdaptiveColor{Light: "#0096C7", Dark: "#0096C7"},
		HeatNegative:       lipgloss.AdaptiveColor{Light: "#B54708", Dark: "#B54708"},
		HeatNegativeMid:    lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#D97706"},
		HeatNegativeStrong: lipgloss.AdaptiveColor{Light: "#F59E0B", Dark: "#F59E0B"},
		HeatNeutral:        lipgloss.AdaptiveColor{Light: "#475467", Dark: "#475467"},
		HeatText:           lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"},
		HeatBorder:         lipgloss.AdaptiveColor{Light: "#263238", Dark: "#171B22"},
	},
}

var Names = []string{"dark", "light", "high contrast", "colorblind safe"}

func For(index int) Palette { return palettes[index%len(palettes)] }

func Title(palette Palette) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(palette.Accent).Padding(0, 1)
}
func Header(palette Palette) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"}).Background(palette.Accent).Padding(0, 1)
}
func Muted(palette Palette) lipgloss.Style { return lipgloss.NewStyle().Foreground(palette.Muted) }
func Panel(palette Palette) lipgloss.Style {
	return lipgloss.NewStyle().Background(palette.Panel).Padding(0, 1)
}

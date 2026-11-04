// Package theme contains terminal styles shared by the TUI views.
package theme

import "github.com/charmbracelet/lipgloss"

type Palette struct {
	Accent lipgloss.AdaptiveColor
	Green  lipgloss.AdaptiveColor
	Red    lipgloss.AdaptiveColor
	Muted  lipgloss.AdaptiveColor
	Panel  lipgloss.AdaptiveColor
}

var Dark = Palette{
	Accent: lipgloss.AdaptiveColor{Light: "#6246EA", Dark: "#8B7CFF"},
	Green:  lipgloss.AdaptiveColor{Light: "#16803C", Dark: "#5CE08A"},
	Red:    lipgloss.AdaptiveColor{Light: "#C62828", Dark: "#FF6B6B"},
	Muted:  lipgloss.AdaptiveColor{Light: "#667085", Dark: "#9299A6"},
	Panel:  lipgloss.AdaptiveColor{Light: "#F1F3F8", Dark: "#20232B"},
}

var palettes = []Palette{
	Dark,
	{
		Accent: lipgloss.AdaptiveColor{Light: "#4937B4", Dark: "#C4BAFF"},
		Green:  lipgloss.AdaptiveColor{Light: "#137333", Dark: "#247A3B"},
		Red:    lipgloss.AdaptiveColor{Light: "#B3261E", Dark: "#B3261E"},
		Muted:  lipgloss.AdaptiveColor{Light: "#4B5563", Dark: "#4B5563"},
		Panel:  lipgloss.AdaptiveColor{Light: "#EEF0F5", Dark: "#EEF0F5"},
	},
	{
		Accent: lipgloss.AdaptiveColor{Light: "#FFFF00", Dark: "#FFFF00"},
		Green:  lipgloss.AdaptiveColor{Light: "#00FF00", Dark: "#00FF00"},
		Red:    lipgloss.AdaptiveColor{Light: "#FF5555", Dark: "#FF5555"},
		Muted:  lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#FFFFFF"},
		Panel:  lipgloss.AdaptiveColor{Light: "#000000", Dark: "#000000"},
	},
	{
		Accent: lipgloss.AdaptiveColor{Light: "#005A9C", Dark: "#79C7FF"},
		Green:  lipgloss.AdaptiveColor{Light: "#0067A5", Dark: "#61B5E8"},
		Red:    lipgloss.AdaptiveColor{Light: "#B54708", Dark: "#FFB05C"},
		Muted:  lipgloss.AdaptiveColor{Light: "#667085", Dark: "#9299A6"},
		Panel:  lipgloss.AdaptiveColor{Light: "#F1F3F8", Dark: "#20232B"},
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

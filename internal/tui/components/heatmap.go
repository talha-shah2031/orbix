package components

import (
	"math"
	"sort"
	"strings"

	"github.com/bubblenexus/bnx/internal/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

// HeatmapTile describes a single heatmap cell. Weight controls its area.
type HeatmapTile struct {
	Name   string
	Detail string
	Weight float64
	Change float64
}

type heatmapRect struct {
	x, y, width, height int
}

// Heatmap draws weighted rectangular tiles and preserves the requested
// terminal dimensions. Tile area is proportional to Weight.
func Heatmap(tiles []HeatmapTile, width, height int, palette theme.Palette) string {
	if width < 1 || height < 1 {
		return ""
	}
	if len(tiles) == 0 {
		return lipgloss.NewStyle().Width(width).Height(height).Foreground(palette.Muted).Render("No Spot market data available")
	}
	if len(tiles) > width*height {
		tiles = tiles[:width*height]
	}
	weights := make([]float64, len(tiles))
	indices := make([]int, len(tiles))
	for index, tile := range tiles {
		weight := tile.Weight
		if weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			weight = 0.01
		}
		weights[index] = weight
		indices[index] = index
	}
	sort.SliceStable(indices, func(i, j int) bool { return weights[indices[i]] > weights[indices[j]] })
	grid := make([][]int, height)
	for row := range grid {
		grid[row] = make([]int, width)
		for column := range grid[row] {
			grid[row][column] = -1
		}
	}
	rects := make([]heatmapRect, len(tiles))
	layoutHeatmap(indices, weights, heatmapRect{width: width, height: height}, rects)
	labels := make([][][]rune, len(tiles))
	for index, rect := range rects {
		for row := rect.y; row < rect.y+rect.height; row++ {
			for column := rect.x; column < rect.x+rect.width; column++ {
				grid[row][column] = index
			}
		}
		labels[index] = heatmapLabel(tiles[index], rect)
		for row := range labels[index] {
			for column, value := range labels[index][row] {
				if value != ' ' {
					grid[rect.y+row][rect.x+column] = index
				}
			}
		}
	}

	var output strings.Builder
	for row := 0; row < height; row++ {
		for column := 0; column < width; {
			index := grid[row][column]
			end := column + 1
			for end < width && grid[row][end] == index {
				end++
			}
			text := make([]rune, end-column)
			for offset := range text {
				text[offset] = ' '
				if index >= 0 {
					rect := rects[index]
					r, c := row-rect.y, column+offset-rect.x
					if r >= 0 && r < len(labels[index]) && c >= 0 && c < len(labels[index][r]) {
						text[offset] = labels[index][r][c]
					}
				}
			}
			segment := string(text)
			if index >= 0 {
				background, foreground := heatmapColors(tiles[index], palette)
				segment = lipgloss.NewStyle().Background(background).Foreground(foreground).Bold(true).Render(segment)
			}
			output.WriteString(segment)
			column = end
		}
		if row+1 < height {
			output.WriteByte('\n')
		}
	}
	return output.String()
}

func heatmapLabel(tile HeatmapTile, rect heatmapRect) [][]rune {
	label := make([][]rune, rect.height)
	for row := range label {
		label[row] = []rune(strings.Repeat(" ", rect.width))
	}
	name := []rune(tile.Name)
	if len(name) > rect.width || rect.height == 0 {
		return label
	}
	lines := [][]rune{name}
	if len(tile.Detail) > 0 && rect.height > 1 && len([]rune(tile.Detail)) <= rect.width {
		lines = append(lines, []rune(tile.Detail))
	}
	startRow := max(0, (rect.height-len(lines))/2)
	for index, text := range lines {
		startColumn := max(0, (rect.width-len(text))/2)
		copy(label[startRow+index][startColumn:], text)
	}
	return label
}

func heatmapColors(tile HeatmapTile, palette theme.Palette) (lipgloss.AdaptiveColor, lipgloss.AdaptiveColor) {
	switch {
	case tile.Change > 0:
		return palette.HeatPositive, palette.HeatText
	case tile.Change < 0:
		return palette.HeatNegative, palette.HeatText
	default:
		return palette.HeatNeutral, palette.HeatText
	}
}

func layoutHeatmap(indices []int, weights []float64, rect heatmapRect, rectangles []heatmapRect) {
	if len(indices) == 0 || rect.width <= 0 || rect.height <= 0 {
		return
	}
	if len(indices) == 1 {
		rectangles[indices[0]] = rect
		return
	}
	vertical := rect.width >= rect.height
	if vertical && rect.width <= 1 {
		vertical = false
	} else if !vertical && rect.height <= 1 {
		vertical = true
	}
	extent := rect.height
	if vertical {
		extent = rect.width
	}
	totalWeight := 0.0
	for _, tileIndex := range indices {
		totalWeight += weights[tileIndex]
	}
	leftWeight := 0.0
	split := 0
	for split < len(indices)-1 {
		leftWeight += weights[indices[split]]
		split++
		if leftWeight >= totalWeight/2 {
			break
		}
	}
	portion := leftWeight / totalWeight
	firstExtent := int(math.Round(float64(extent) * portion))
	firstExtent = max(1, min(extent-1, firstExtent))
	firstCapacity, secondCapacity := firstExtent, extent-firstExtent
	if vertical {
		firstCapacity *= rect.height
		secondCapacity *= rect.height
	} else {
		firstCapacity *= rect.width
		secondCapacity *= rect.width
	}
	split = max(len(indices)-secondCapacity, min(split, firstCapacity))
	split = max(1, min(len(indices)-1, split))
	first := rect
	second := rect
	if vertical {
		first.width = firstExtent
		second.x += firstExtent
		second.width -= firstExtent
	} else {
		first.height = firstExtent
		second.y += firstExtent
		second.height -= firstExtent
	}
	layoutHeatmap(indices[:split], weights, first, rectangles)
	layoutHeatmap(indices[split:], weights, second, rectangles)
}

package components

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/bubblenexus/bnx/internal/tui/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const minTreemapTileWidth = 5

type HeatmapTile struct {
	Name   string
	Detail string
	Weight float64
	Change float64
}

type HeatmapMode uint8

const (
	HeatmapAuto HeatmapMode = iota
	HeatmapTreemap
	HeatmapGrid
)

type heatmapRect struct {
	x, y, width, height int
}

type heatmapCell struct {
	rune rune
	fg   lipgloss.AdaptiveColor
	bg   lipgloss.AdaptiveColor
	bold bool
	set  bool
}

// HeatmapRenderer retains tile rectangles while the size and ranked coin
// order stay unchanged. Price and volume updates still repaint cell colors.
type HeatmapRenderer struct {
	width, height int
	orderKey      string
	grid          bool
	rectangles    []heatmapRect
}

func UsesHeatmapGrid(width, height int, mode HeatmapMode) bool {
	switch mode {
	case HeatmapGrid:
		return true
	case HeatmapTreemap:
		return false
	case HeatmapAuto:
		return width < 120 || height < 35
	default:
		return width < 120 || height < 35
	}
}

func (renderer *HeatmapRenderer) Render(tiles []HeatmapTile, width, height, terminalWidth, terminalHeight int, palette theme.Palette, mode HeatmapMode, scrollRows int) string {
	if width < 1 || height < 1 {
		return ""
	}
	ordered := append([]HeatmapTile(nil), tiles...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Weight == ordered[j].Weight {
			return ordered[i].Name < ordered[j].Name
		}
		return ordered[i].Weight > ordered[j].Weight
	})
	gridMode := UsesHeatmapGrid(terminalWidth, terminalHeight, mode)
	key := heatmapOrderKey(ordered)
	if renderer.width != width || renderer.height != height || renderer.orderKey != key || renderer.grid != gridMode {
		renderer.width, renderer.height = width, height
		renderer.orderKey, renderer.grid = key, gridMode
		if gridMode {
			renderer.rectangles = uniformGridRects(len(ordered), width, height)
		} else {
			renderer.rectangles = layoutHeatmap(ordered, width, height)
			if !validHeatmapLayout(renderer.rectangles, len(ordered), width, height) {
				gridMode = true
				renderer.grid = true
				renderer.rectangles = uniformGridRects(len(ordered), width, height)
			}
		}
	}
	if gridMode {
		return renderGrid(ordered, renderer.rectangles, width, height, palette, scrollRows)
	}
	return renderTreemap(ordered, renderer.rectangles, width, height, palette)
}

func heatmapOrderKey(tiles []HeatmapTile) string {
	var key strings.Builder
	for _, tile := range tiles {
		key.WriteString(tile.Name)
		key.WriteByte(0)
	}
	return key.String()
}

func HeatmapGridColumns(availableWidth int) int {
	if availableWidth < 1 {
		return 1
	}
	return max(1, min(10, availableWidth/9))
}

func renderGrid(tiles []HeatmapTile, rects []heatmapRect, width, height int, palette theme.Palette, scrollRows int) string {
	visible := make([]heatmapRect, len(rects))
	columns := HeatmapGridColumns(width)
	cellHeight := 2
	if len(rects) > 0 {
		cellHeight = rects[0].height
	}
	rowsVisible := max(1, (height+cellHeight-1)/cellHeight)
	totalRows := 0
	if columns > 0 {
		totalRows = (len(rects) + columns - 1) / columns
	}
	maximumOffset := max(0, totalRows-rowsVisible)
	scrollRows = max(0, min(scrollRows, maximumOffset))
	for index, rect := range rects {
		rect.y -= scrollRows * cellHeight
		visible[index] = rect
	}
	return renderCells(tiles, visible, width, height, palette, true)
}

func HeatmapGridStatus(count, width, height, scrollRows int) string {
	columns := HeatmapGridColumns(width)
	rows := (count + columns - 1) / columns
	cellHeight := 2
	if rows*3 <= height {
		cellHeight = 3
	}
	visible := max(1, (height+cellHeight-1)/cellHeight)
	start := max(0, min(scrollRows, max(0, rows-visible)))
	end := min(rows, start+visible)
	if rows == 0 {
		return "rows 0-0 / 0"
	}
	return fmt.Sprintf("rows %d-%d / %d", start+1, end, rows)
}

func uniformGridRects(count, width, height int) []heatmapRect {
	columns := HeatmapGridColumns(width)
	rows := (count + columns - 1) / columns
	cellHeight := 2
	if rows*3 <= height {
		cellHeight = 3
	}
	if rows == 0 {
		return nil
	}
	baseWidth, remainder := width/columns, width%columns
	widths := make([]int, columns)
	xPositions := make([]int, columns)
	x := 0
	for column := 0; column < columns; column++ {
		widths[column] = baseWidth
		if column < remainder {
			widths[column]++
		}
		xPositions[column] = x
		x += widths[column]
	}
	rects := make([]heatmapRect, rows*columns)
	for index := range rects {
		column, row := index%columns, index/columns
		rects[index] = heatmapRect{x: xPositions[column], y: row * cellHeight, width: widths[column], height: cellHeight}
	}
	return rects
}

func CapHeatmapWeights(raw []float64, maximum float64) []float64 {
	if len(raw) == 0 {
		return nil
	}
	maximum = max(1/float64(len(raw)), min(1, maximum))
	weights := make([]float64, len(raw))
	active := make([]bool, len(raw))
	for index, value := range raw {
		if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			value = 0.0001
		}
		weights[index] = value
		active[index] = true
	}
	remaining := 1.0
	for {
		total := 0.0
		for index, enabled := range active {
			if enabled {
				total += weights[index]
			}
		}
		if total == 0 {
			break
		}
		var cappedIndices []int
		for index, enabled := range active {
			if enabled && weights[index]/total*remaining > maximum {
				cappedIndices = append(cappedIndices, index)
			}
		}
		if len(cappedIndices) == 0 {
			for index, enabled := range active {
				if enabled {
					weights[index] = weights[index] / total * remaining
				}
			}
			break
		}
		for _, index := range cappedIndices {
			weights[index] = maximum
			active[index] = false
			remaining -= maximum
		}
	}
	return weights
}

func renderTreemap(tiles []HeatmapTile, rects []heatmapRect, width, height int, palette theme.Palette) string {
	return renderCells(tiles, rects, width, height, palette, false)
}

func renderCells(tiles []HeatmapTile, rects []heatmapRect, width, height int, palette theme.Palette, gridMode bool) string {
	buffer := make([][]heatmapCell, height)
	for y := range buffer {
		buffer[y] = make([]heatmapCell, width)
	}
	for index, rect := range rects {
		if index >= len(tiles) || rect.width <= 0 || rect.height <= 0 || rect.y >= height || rect.y+rect.height <= 0 {
			continue
		}
		background, foreground := heatmapColors(tiles[index], palette)
		edge := edgeColor(background, palette.HeatBorder)
		fillRect(buffer, rect, width, height, foreground, background, edge)
		labels := TileLabel(tiles[index].Name, tiles[index].Detail, rect.width, rect.height)
		// Terminal cells have a fixed font size, so use bold emphasis for labels
		// in larger tiles and regular weight where the available area is tight.
		bold := rect.width >= 14 && rect.height >= 3
		if gridMode && rect.height > 1 {
			labels = []string{runewidth.Truncate(tiles[index].Name, rect.width, "…"), runewidth.Truncate(tiles[index].Detail, rect.width, "…")}
		}
		labelY := rect.y + (rect.height-len(labels))/2
		for lineIndex, label := range labels {
			if label == "" {
				continue
			}
			lineWidth := runewidth.StringWidth(label)
			labelX := rect.x + (rect.width-lineWidth)/2
			writeCellText(buffer, labelX, labelY+lineIndex, label, rect, width, height, foreground, background, edge, bold)
		}
	}
	return renderCellBuffer(buffer, width, height)
}

func fillRect(buffer [][]heatmapCell, rect heatmapRect, width, height int, fg, bg, edge lipgloss.AdaptiveColor) {
	x0, x1 := max(0, rect.x), min(width, rect.x+rect.width)
	y0, y1 := max(0, rect.y), min(height, rect.y+rect.height)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			cellBG := bg
			if heatmapEdgeCell(x, y, rect) {
				cellBG = edge
			}
			buffer[y][x] = heatmapCell{rune: ' ', fg: fg, bg: cellBG, set: true}
		}
	}
}

func writeCellText(buffer [][]heatmapCell, x, y int, text string, rect heatmapRect, width, height int, fg, bg, edge lipgloss.AdaptiveColor, bold bool) {
	if y < 0 || y >= height {
		return
	}
	for _, symbol := range text {
		cellWidth := runewidth.RuneWidth(symbol)
		if cellWidth < 1 {
			continue
		}
		if x < rect.x || x+cellWidth > rect.x+rect.width || x+cellWidth > width {
			return
		}
		cellBG := bg
		if heatmapEdgeCell(x, y, rect) || heatmapEdgeCell(x+cellWidth-1, y, rect) {
			cellBG = edge
		}
		buffer[y][x] = heatmapCell{rune: symbol, fg: fg, bg: cellBG, bold: bold, set: true}
		if cellWidth == 2 {
			buffer[y][x+1] = heatmapCell{fg: fg, bg: cellBG, bold: bold, set: true}
		}
		x += cellWidth
	}
}

func heatmapEdgeCell(x, y int, rect heatmapRect) bool {
	// Draw only the right and bottom edges. Since treemap rectangles share
	// boundaries, this leaves a single dark separator instead of a two-cell
	// border where neighboring tiles each draw all four sides.
	return rect.width >= 2 && x == rect.x+rect.width-1 || rect.height >= 2 && y == rect.y+rect.height-1
}

// TileLabel chooses a label that fits the tile using terminal display cells.
func TileLabel(symbol, change string, width, height int) []string {
	if width < 1 || height < 1 {
		return nil
	}
	if width >= 8 && height >= 2 {
		return []string{
			runewidth.Truncate(symbol, width, "…"),
			runewidth.Truncate(change, width, "…"),
		}
	}
	if runewidth.StringWidth(symbol)+1 <= width {
		return []string{symbol}
	}
	return nil
}

func renderCellBuffer(buffer [][]heatmapCell, width, height int) string {
	var output strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; {
			cell := buffer[y][x]
			if !cell.set {
				output.WriteByte(' ')
				x++
				continue
			}
			end := x + 1
			for end < width && sameCellStyle(cell, buffer[y][end]) {
				end++
			}
			var text strings.Builder
			for column := x; column < end; column++ {
				r := buffer[y][column].rune
				if r != 0 {
					text.WriteRune(r)
				}
			}
			segmentWidth := end - x
			output.WriteString(lipgloss.NewStyle().Foreground(cell.fg).Background(cell.bg).Bold(cell.bold).Width(segmentWidth).MaxWidth(segmentWidth).MaxHeight(1).Render(text.String()))
			x = end
		}
		if y+1 < height {
			output.WriteByte('\n')
		}
	}
	return output.String()
}

func sameCellStyle(a, b heatmapCell) bool {
	return b.set && a.fg == b.fg && a.bg == b.bg && a.bold == b.bold
}

func heatmapColors(tile HeatmapTile, palette theme.Palette) (lipgloss.AdaptiveColor, lipgloss.AdaptiveColor) {
	change := max(-10, min(10, tile.Change)) / 10
	if change < 0 {
		return blendColor(palette.HeatNeutral, palette.HeatNegativeStrong, -change), palette.HeatText
	}
	return blendColor(palette.HeatNeutral, palette.HeatPositiveStrong, change), palette.HeatText
}

func edgeColor(background, edge lipgloss.AdaptiveColor) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{
		Light: blendHex(background.Light, edge.Light, 0.55),
		Dark:  blendHex(background.Dark, edge.Dark, 0.55),
	}
}

func blendColor(from, to lipgloss.AdaptiveColor, ratio float64) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: blendHex(from.Light, to.Light, ratio), Dark: blendHex(from.Dark, to.Dark, ratio)}
}

func blendHex(from, to string, ratio float64) string {
	parse := func(value string) (int64, int64, int64) {
		value = strings.TrimPrefix(value, "#")
		if len(value) != 6 {
			return 0, 0, 0
		}
		color, err := strconv.ParseInt(value, 16, 32)
		if err != nil {
			return 0, 0, 0
		}
		return color >> 16, (color >> 8) & 0xff, color & 0xff
	}
	fr, fg, fb := parse(from)
	tr, tg, tb := parse(to)
	component := func(a, b int64) int64 { return int64(math.Round(float64(a) + (float64(b)-float64(a))*ratio)) }
	return fmt.Sprintf("#%02X%02X%02X", component(fr, tr), component(fg, tg), component(fb, tb))
}

func layoutHeatmap(tiles []HeatmapTile, width, height int) []heatmapRect {
	if width < minTreemapTileWidth || height < 1 || len(tiles) == 0 || len(tiles) > width/minTreemapTileWidth*height {
		return nil
	}
	weights := make([]float64, len(tiles))
	indices := make([]int, len(tiles))
	for i, tile := range tiles {
		weights[i] = tile.Weight
		if weights[i] <= 0 || math.IsNaN(weights[i]) || math.IsInf(weights[i], 0) {
			weights[i] = 0.01
		}
		indices[i] = i
	}
	rects := make([]heatmapRect, len(tiles))
	layoutHeatmapRange(indices, weights, heatmapRect{width: width, height: height}, rects)
	return rects
}

func layoutHeatmapRange(indices []int, weights []float64, rect heatmapRect, rectangles []heatmapRect) {
	if len(indices) == 0 || rect.width < minTreemapTileWidth || rect.height < 1 {
		return
	}
	if len(indices) == 1 {
		rectangles[indices[0]] = rect
		return
	}
	totalWeight := 0.0
	for _, index := range indices {
		totalWeight += weights[index]
	}
	// cmd.exe and Windows Terminal cells are taller than they are wide. Compare
	// the visual width (half the terminal-cell width) to the height when choosing
	// slice direction so the resulting blocks are closer to square on screen.
	vertical := rect.width >= 2*rect.height && rect.width >= 2*minTreemapTileWidth
	leftWeight, preferredSplit := 0.0, 0
	for preferredSplit < len(indices)-1 {
		leftWeight += weights[indices[preferredSplit]]
		preferredSplit++
		if leftWeight >= totalWeight/2 {
			break
		}
	}
	split, firstExtent := feasibleSplit(indices, weights, rect, vertical, preferredSplit, totalWeight)
	if split == 0 && vertical {
		vertical = false
		split, firstExtent = feasibleSplit(indices, weights, rect, vertical, preferredSplit, totalWeight)
	}
	if split == 0 {
		return
	}
	first, second := rect, rect
	if vertical {
		first.width = firstExtent
		second.x += firstExtent
		second.width -= firstExtent
	} else {
		first.height = firstExtent
		second.y += firstExtent
		second.height -= firstExtent
	}
	layoutHeatmapRange(indices[:split], weights, first, rectangles)
	layoutHeatmapRange(indices[split:], weights, second, rectangles)
}

func feasibleSplit(indices []int, weights []float64, rect heatmapRect, vertical bool, preferred int, totalWeight float64) (int, int) {
	bestSplit, bestExtent := 0, 0
	bestDistance := math.MaxFloat64
	weightBefore := 0.0
	for split := 1; split < len(indices); split++ {
		weightBefore += weights[indices[split-1]]
		var minExtent, maxExtent int
		if vertical {
			minExtent = minTreemapTileWidth * ((split + rect.height - 1) / rect.height)
			maxExtent = rect.width - minTreemapTileWidth*((len(indices)-split+rect.height-1)/rect.height)
		} else {
			columns := rect.width / minTreemapTileWidth
			minExtent = (split + columns - 1) / columns
			maxExtent = rect.height - (len(indices)-split+columns-1)/columns
		}
		if minExtent > maxExtent {
			continue
		}
		portion := weightBefore / totalWeight
		extent := rect.height
		if vertical {
			extent = rect.width
		}
		origin := rect.y
		if vertical {
			origin = rect.x
		}
		firstExtent := cumulativeEdge(origin, extent, weightBefore, totalWeight) - origin
		firstExtent = max(minExtent, min(maxExtent, firstExtent))
		distance := math.Abs(portion - 0.5)
		if split == preferred {
			distance--
		}
		if distance < bestDistance {
			bestDistance, bestSplit, bestExtent = distance, split, firstExtent
		}
	}
	return bestSplit, bestExtent
}

func cumulativeEdge(origin, extent int, weightBefore, totalWeight float64) int {
	if totalWeight <= 0 {
		return origin
	}
	return origin + int(math.Round(float64(extent)*weightBefore/totalWeight))
}

func validHeatmapLayout(rectangles []heatmapRect, count, width, height int) bool {
	if len(rectangles) != count || width < minTreemapTileWidth || height < 1 {
		return false
	}
	covered := make([][]bool, height)
	for y := range covered {
		covered[y] = make([]bool, width)
	}
	area := 0
	for _, rect := range rectangles {
		if rect.width < minTreemapTileWidth || rect.height < 1 || rect.x < 0 || rect.y < 0 || rect.x+rect.width > width || rect.y+rect.height > height {
			return false
		}
		area += rect.width * rect.height
		for y := rect.y; y < rect.y+rect.height; y++ {
			for x := rect.x; x < rect.x+rect.width; x++ {
				if covered[y][x] {
					return false
				}
				covered[y][x] = true
			}
		}
	}
	return area == width*height
}

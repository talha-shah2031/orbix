package components

import (
	"fmt"
	"strings"
	"testing"

	"github.com/talha-shah2031/orbix/internal/tui/theme"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

func TestLayoutHeatmapCoversGridAndRespectsMinimumWidth(t *testing.T) {
	for _, size := range []struct{ width, height int }{{120, 27}, {200, 42}} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			tiles := make([]HeatmapTile, 100)
			for i := range tiles {
				tiles[i] = HeatmapTile{Name: fmt.Sprintf("C%d", i), Weight: 0.85 / 99}
			}
			tiles[0].Weight = 0.15
			rects := layoutHeatmap(tiles, size.width, size.height)
			if len(rects) != len(tiles) {
				t.Fatalf("got %d rectangles for %d tiles", len(rects), len(tiles))
			}
			covered := make([][]bool, size.height)
			for y := range covered {
				covered[y] = make([]bool, size.width)
			}
			area := 0
			largestArea := 0
			for index, rect := range rects {
				if rect.width < minTreemapTileWidth || rect.height < 1 {
					t.Fatalf("tile %d is smaller than minimum: %+v", index, rect)
				}
				if rect.x < 0 || rect.y < 0 || rect.x+rect.width > size.width || rect.y+rect.height > size.height {
					t.Fatalf("tile %d is outside bounds: %+v", index, rect)
				}
				tileArea := rect.width * rect.height
				area += tileArea
				largestArea = max(largestArea, tileArea)
				for y := rect.y; y < rect.y+rect.height; y++ {
					for x := rect.x; x < rect.x+rect.width; x++ {
						if covered[y][x] {
							t.Fatalf("tile %d overlaps another tile at (%d,%d)", index, x, y)
						}
						covered[y][x] = true
					}
				}
			}
			if area != size.width*size.height {
				t.Fatalf("rectangle area sum = %d, want %d", area, size.width*size.height)
			}
			if !validHeatmapLayout(rects, len(tiles), size.width, size.height) {
				t.Fatal("layout validator rejected a full-area tile partition")
			}
			if float64(largestArea)/float64(area) > 0.16 {
				t.Fatalf("largest tile occupies %.1f%% of area; expected approximately 15%% or less", 100*float64(largestArea)/float64(area))
			}
			for y := range covered {
				for x, present := range covered[y] {
					if !present {
						t.Fatalf("uncovered cell at (%d,%d)", x, y)
					}
				}
			}
		})
	}
}

func TestLayoutHeatmapRejectsSizeThatCannotFitMinimumTiles(t *testing.T) {
	tiles := make([]HeatmapTile, 10)
	if rects := layoutHeatmap(tiles, 20, 2); rects != nil {
		t.Fatalf("expected no treemap layout for impossible 5-cell minimum, got %d rectangles", len(rects))
	}
}

func TestCapHeatmapWeightsLimitsOutlierShare(t *testing.T) {
	raw := make([]float64, 100)
	for i := range raw {
		raw[i] = 1
	}
	raw[0] = 1000
	weights := CapHeatmapWeights(raw, 0.15)
	if len(weights) != len(raw) {
		t.Fatalf("got %d weights, want %d", len(weights), len(raw))
	}
	total := 0.0
	for _, weight := range weights {
		total += weight
	}
	if total < 0.999999 || total > 1.000001 {
		t.Fatalf("normalized weight sum = %f, want 1", total)
	}
	if weights[0] > 0.150001 {
		t.Fatalf("outlier weight = %f, want <= 0.15", weights[0])
	}
}

func TestTileLabelDisplayWidthCases(t *testing.T) {
	tests := []struct {
		name, symbol, change string
		width, height        int
		wantLines            int
	}{
		{name: "full label at width eight", symbol: "BTC", change: "+2.31%", width: 8, height: 2, wantLines: 2},
		{name: "symbol only", symbol: "BTC", change: "+2.31%", width: 5, height: 1, wantLines: 1},
		{name: "too narrow", symbol: "BTC", change: "+2.31%", width: 3, height: 1, wantLines: 0},
		{name: "CJK symbol fits by display width", symbol: "牛来", change: "+2.31%", width: 5, height: 1, wantLines: 1},
		{name: "CJK symbol does not fit", symbol: "牛来", change: "+2.31%", width: 4, height: 1, wantLines: 0},
		{name: "CJK full label", symbol: "牛来", change: "+2.31%", width: 9, height: 2, wantLines: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lines := TileLabel(test.symbol, test.change, test.width, test.height)
			if len(lines) != test.wantLines {
				t.Fatalf("got %d lines (%q), want %d", len(lines), lines, test.wantLines)
			}
			for _, line := range lines {
				if lineWidth := runewidth.StringWidth(line); lineWidth > test.width {
					t.Fatalf("label %q has display width %d, tile width is %d", line, lineWidth, test.width)
				}
			}
		})
	}
}

func TestHeatmapRenderHasExactDisplayDimensionsWithCJK(t *testing.T) {
	tiles := []HeatmapTile{
		{Name: "牛来", Detail: "+2.31%", Weight: 0.5, Change: 2.31},
		{Name: "BTC", Detail: "-1.00%", Weight: 0.3, Change: -1},
		{Name: "ETH", Detail: "+0.10%", Weight: 0.2, Change: 0.1},
	}
	var renderer HeatmapRenderer
	output := renderer.Render(tiles, 40, 8, 120, 35, theme.Dark, HeatmapTreemap, 0)
	lines := strings.Split(ansi.Strip(output), "\n")
	if len(lines) != 8 {
		t.Fatalf("rendered %d rows, want 8", len(lines))
	}
	for row, line := range lines {
		if width := runewidth.StringWidth(line); width != 40 {
			t.Fatalf("row %d display width = %d, want 40", row, width)
		}
	}
}

func TestHeatmapGridModeFitsAndScrollsAtSmallSizes(t *testing.T) {
	tiles := make([]HeatmapTile, 100)
	for i := range tiles {
		tiles[i] = HeatmapTile{Name: fmt.Sprintf("C%d", i), Detail: "+1.23%", Weight: 1, Change: 1}
	}
	var renderer HeatmapRenderer
	for _, scroll := range []int{0, 4, 100} {
		output := renderer.Render(tiles, 80, 13, 80, 24, theme.Dark, HeatmapAuto, scroll)
		lines := strings.Split(ansi.Strip(output), "\n")
		if len(lines) != 13 {
			t.Fatalf("scroll %d rendered %d rows, want 13", scroll, len(lines))
		}
		for row, line := range lines {
			if width := runewidth.StringWidth(line); width != 80 {
				t.Fatalf("scroll %d row %d display width = %d, want 80", scroll, row, width)
			}
		}
	}
}

func TestHeatmapGridColumnsAtRequiredWidths(t *testing.T) {
	tests := []struct{ width, columns int }{
		{60, 6}, {80, 8}, {100, 10}, {120, 10}, {160, 10}, {200, 10},
	}
	for _, test := range tests {
		if got := HeatmapGridColumns(test.width); got != test.columns {
			t.Errorf("HeatmapGridColumns(%d) = %d, want %d", test.width, got, test.columns)
		}
	}
}

func TestHeatmapGridFillsWidthAtRequiredTerminalSizes(t *testing.T) {
	tiles := make([]HeatmapTile, 100)
	for i := range tiles {
		tiles[i] = HeatmapTile{Name: fmt.Sprintf("C%d", i), Detail: "+1.23%", Weight: 1, Change: 1}
	}
	for _, width := range []int{60, 80, 100, 120, 160, 200} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			columns := HeatmapGridColumns(width)
			rects := uniformGridRects(len(tiles), width, 24)
			if len(rects)%columns != 0 {
				t.Fatalf("rectangle count %d does not contain complete rows of %d", len(rects), columns)
			}
			x := 0
			for column := 0; column < columns; column++ {
				rect := rects[column]
				if rect.x != x || rect.width < 9 {
					t.Fatalf("column %d starts at %d width %d; expected contiguous cell >=9 at %d", column, rect.x, rect.width, x)
				}
				x += rect.width
			}
			if x != width {
				t.Fatalf("grid ends at column %d, want terminal width %d", x, width)
			}
			var renderer HeatmapRenderer
			output := renderer.Render(tiles, width, 12, width, 24, theme.Dark, HeatmapGrid, 0)
			for row, line := range strings.Split(ansi.Strip(output), "\n") {
				if got := runewidth.StringWidth(line); got != width {
					t.Fatalf("row %d display width %d, want %d", row, got, width)
				}
			}
		})
	}
}

func TestHeatmapGridStatusReportsVisibleRows(t *testing.T) {
	if got := HeatmapGridStatus(100, 80, 15, 0); got != "rows 1-8 / 13" {
		t.Fatalf("status = %q, want rows 1-8 / 13", got)
	}
	if got := HeatmapGridStatus(100, 80, 15, 100); got != "rows 6-13 / 13" {
		t.Fatalf("clamped status = %q, want rows 6-13 / 13", got)
	}
}

func TestUniformGridDistributesWidthAndPadsLastRow(t *testing.T) {
	const width, height, count = 101, 60, 101
	rects := uniformGridRects(count, width, height)
	columns := HeatmapGridColumns(width)
	if len(rects)%columns != 0 {
		t.Fatalf("grid has %d rectangles for %d columns; final row is incomplete", len(rects), columns)
	}
	if len(rects) != 110 {
		t.Fatalf("got %d rectangles, want 110 including empty padding", len(rects))
	}
	if rects[0].width != 11 || rects[1].width != 10 || rects[9].x+rects[9].width != width {
		t.Fatalf("grid widths do not distribute remainder or fill terminal width: first=%d second=%d end=%d", rects[0].width, rects[1].width, rects[9].x+rects[9].width)
	}
	if rects[0].height != 3 {
		t.Fatalf("cell height = %d, want 3 when all rows fit", rects[0].height)
	}
}

func TestHeatmapLayoutCacheTracksResizeAndRankedOrder(t *testing.T) {
	tiles := []HeatmapTile{{Name: "BTC", Weight: 0.6}, {Name: "ETH", Weight: 0.4}}
	var renderer HeatmapRenderer
	renderer.Render(tiles, 120, 27, 120, 35, theme.Dark, HeatmapAuto, 0)
	firstRects := append([]heatmapRect(nil), renderer.rectangles...)
	firstKey := renderer.orderKey
	tiles[0].Weight, tiles[1].Weight = 0.55, 0.45
	renderer.Render(tiles, 120, 27, 120, 35, theme.Dark, HeatmapAuto, 0)
	if !equalRects(firstRects, renderer.rectangles) {
		t.Fatal("layout recalculated despite unchanged ranked order and terminal size")
	}
	tiles[0].Weight, tiles[1].Weight = 0.4, 0.6
	renderer.Render(tiles, 120, 27, 120, 35, theme.Dark, HeatmapAuto, 0)
	if renderer.orderKey == firstKey {
		t.Fatal("layout cache did not update after ranked order changed")
	}
	renderer.Render(tiles, 200, 42, 200, 50, theme.Dark, HeatmapAuto, 0)
	if renderer.width != 200 || renderer.height != 42 {
		t.Fatalf("cached dimensions = %dx%d, want 200x42", renderer.width, renderer.height)
	}
}

func equalRects(left, right []heatmapRect) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func TestUsesHeatmapGridForSmallTerminalsAndManualMode(t *testing.T) {
	if !UsesHeatmapGrid(119, 50, HeatmapAuto) || !UsesHeatmapGrid(200, 34, HeatmapAuto) {
		t.Fatal("small terminal should use grid mode")
	}
	if UsesHeatmapGrid(120, 35, HeatmapAuto) {
		t.Fatal("120x35 should use treemap mode")
	}
	if !UsesHeatmapGrid(200, 50, HeatmapGrid) || UsesHeatmapGrid(80, 24, HeatmapTreemap) {
		t.Fatal("manual layout override was ignored")
	}
}

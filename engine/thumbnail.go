package engine

import (
	"image"
	"image/color"
	"image/png"
	"io"
)

// ThumbnailConfig configures thumbnail rendering.
type ThumbnailConfig struct {
	Width       int
	Height      int
	CellSize    int
	Background  color.Color
	WallColor   color.Color
	GridColor   color.Color
	PlayerColors []color.Color
	EnergyColor color.Color
	CoreColor   color.Color
}

// DefaultThumbnailConfig returns the default thumbnail configuration.
func DefaultThumbnailConfig() ThumbnailConfig {
	return ThumbnailConfig{
		Width:      640,
		Height:     360,
		CellSize:   6,
		Background: color.RGBA{18, 18, 24, 255},
		WallColor:  color.RGBA{60, 60, 70, 255},
		GridColor:  color.RGBA{30, 30, 40, 255},
		PlayerColors: []color.Color{
			color.RGBA{66, 165, 245, 255}, // Blue
			color.RGBA{239, 83, 80, 255},  // Red
			color.RGBA{102, 187, 106, 255}, // Green
			color.RGBA{255, 202, 40, 255},  // Yellow
			color.RGBA{171, 71, 188, 255},  // Purple
			color.RGBA{255, 112, 67, 255},  // Orange
			color.RGBA{38, 198, 218, 255},  // Cyan
			color.RGBA{236, 64, 122, 255},  // Pink
		},
		EnergyColor: color.RGBA{255, 235, 59, 255},
		CoreColor:   color.RGBA{255, 255, 255, 255},
	}
}

// RenderThumbnail renders a replay turn as a PNG thumbnail.
func RenderThumbnail(replay *Replay, turnNum int, cfg ThumbnailConfig) (*image.RGBA, error) {
	if turnNum < 0 || turnNum >= len(replay.Turns) {
		turnNum = len(replay.Turns) - 1
	}

	turn := replay.Turns[turnNum]
	img := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))

	drawBackground(img, cfg.Background)

	cellW := float64(cfg.Width) / float64(replay.Map.Cols)
	cellH := float64(cfg.Height) / float64(replay.Map.Rows)

	drawGrid(img, replay.Map.Cols, replay.Map.Rows, cellW, cellH, cfg.GridColor)
	drawWalls(img, replay.Map.Walls, replay.Map.Cols, cellW, cellH, cfg.WallColor)
	drawEnergy(img, turn.Energy, replay.Map.Cols, cellW, cellH, cfg.EnergyColor)
	drawCores(img, turn.Cores, replay.Map.Cols, cellW, cellH, cfg.CoreColor, cfg.PlayerColors)
	drawBots(img, turn.Bots, replay.Map.Cols, cellW, cellH, cfg.PlayerColors)

	return img, nil
}

// RenderThumbnailPNG renders a replay turn and writes it as PNG.
func RenderThumbnailPNG(replay *Replay, turnNum int, w io.Writer) error {
	cfg := DefaultThumbnailConfig()
	img, err := RenderThumbnail(replay, turnNum, cfg)
	if err != nil {
		return err
	}
	return png.Encode(w, img)
}

// RenderFinalTurnThumbnail renders the final turn of a replay as PNG.
func RenderFinalTurnThumbnail(replay *Replay, w io.Writer) error {
	return RenderThumbnailPNG(replay, len(replay.Turns)-1, w)
}

// RenderMidGameThumbnail renders the mid-game turn (40% through) as PNG.
func RenderMidGameThumbnail(replay *Replay, w io.Writer) error {
	turnNum := len(replay.Turns) * 2 / 5
	if turnNum >= len(replay.Turns) {
		turnNum = len(replay.Turns) - 1
	}
	cfg := DefaultThumbnailConfig()
	img, err := RenderThumbnail(replay, turnNum, cfg)
	if err != nil {
		return err
	}
	return png.Encode(w, img)
}

func drawBackground(img *image.RGBA, c color.Color) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.Set(x, y, c)
		}
	}
}

func drawGrid(img *image.RGBA, cols, rows int, cellW, cellH float64, c color.Color) {
	bounds := img.Bounds()

	for col := 0; col <= cols; col++ {
		x := int(float64(col) * cellW)
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			img.Set(x, y, c)
		}
	}

	for row := 0; row <= rows; row++ {
		y := int(float64(row) * cellH)
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.Set(x, y, c)
		}
	}
}

func drawWalls(img *image.RGBA, walls []Position, cols int, cellW, cellH float64, c color.Color) {
	for _, w := range walls {
		x0 := int(float64(w.Col) * cellW)
		y0 := int(float64(w.Row) * cellH)
		x1 := int(float64(w.Col+1) * cellW)
		y1 := int(float64(w.Row+1) * cellH)

		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.Set(x, y, c)
			}
		}
	}
}

func drawBots(img *image.RGBA, bots []ReplayBot, cols int, cellW, cellH float64, colors []color.Color) {
	for _, b := range bots {
		if !b.Alive {
			continue
		}

		c := colors[b.Owner%len(colors)]
		x0 := int(float64(b.Position.Col)*cellW + cellW*0.25)
		y0 := int(float64(b.Position.Row)*cellH + cellH*0.25)
		x1 := int(float64(b.Position.Col+1)*cellW - cellW*0.25)
		y1 := int(float64(b.Position.Row+1)*cellH - cellH*0.25)

		cx, cy := (x0+x1)/2, (y0+y1)/2
		r := (x1 - x0)
		if r > (y1 - y0) {
			r = y1 - y0
		}

		drawCircle(img, cx, cy, r/2, c)
	}
}

func drawCores(img *image.RGBA, cores []ReplayCoreState, cols int, cellW, cellH float64, baseColor color.Color, playerColors []color.Color) {
	for _, c := range cores {
		x0 := int(float64(c.Position.Col)*cellW + cellW*0.15)
		y0 := int(float64(c.Position.Row)*cellH + cellH*0.15)
		x1 := int(float64(c.Position.Col+1)*cellW - cellW*0.15)
		y1 := int(float64(c.Position.Row+1)*cellH - cellH*0.15)

		fillColor := baseColor
		if c.Active {
			fillColor = playerColors[c.Owner%len(playerColors)]
		}

		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.Set(x, y, fillColor)
			}
		}

		strokeColor := color.RGBA{0, 0, 0, 255}
		drawRectOutline(img, x0, y0, x1, y1, strokeColor)
	}
}

func drawEnergy(img *image.RGBA, energy []Position, cols int, cellW, cellH float64, c color.Color) {
	for _, e := range energy {
		cx := int((float64(e.Col) + 0.5) * cellW)
		cy := int((float64(e.Row) + 0.5) * cellH)
		r := int(cellW * 0.3)

		drawDiamond(img, cx, cy, r, c)
	}
}

func drawCircle(img *image.RGBA, cx, cy, r int, c color.Color) {
	r2 := r * r
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r2 {
				img.Set(cx+dx, cy+dy, c)
			}
		}
	}
}

func drawDiamond(img *image.RGBA, cx, cy, r int, c color.Color) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if abs(dx)+abs(dy) <= r {
				img.Set(cx+dx, cy+dy, c)
			}
		}
	}
}

func drawRectOutline(img *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	for x := x0; x < x1; x++ {
		img.Set(x, y0, c)
		img.Set(x, y1-1, c)
	}
	for y := y0; y < y1; y++ {
		img.Set(x0, y, c)
		img.Set(x1-1, y, c)
	}
}

// SelectThumbnailTurn selects the most interesting turn for thumbnail generation.
// Prioritizes: end game > mid game with many bots > late game.
func SelectThumbnailTurn(replay *Replay) int {
	if len(replay.Turns) == 0 {
		return 0
	}

	finalTurn := len(replay.Turns) - 1
	if finalTurn < 10 {
		return finalTurn
	}

	maxBotsTurn := finalTurn
	maxBots := 0
	midGame := replay.Turns[finalTurn].Turn * 2 / 5

	for i := len(replay.Turns) - 1; i >= 0; i-- {
		botCount := 0
		for _, b := range replay.Turns[i].Bots {
			if b.Alive {
				botCount++
			}
		}
		if botCount > maxBots {
			maxBots = botCount
			maxBotsTurn = i
		}
		if replay.Turns[i].Turn <= midGame {
			break
		}
	}

	if maxBotsTurn > finalTurn*3/4 {
		return finalTurn
	}
	return maxBotsTurn
}

// GenerateMatchThumbnail generates a thumbnail image for a match.
// Uses the most interesting turn as determined by SelectThumbnailTurn.
func GenerateMatchThumbnail(replay *Replay) (*image.RGBA, error) {
	turnNum := SelectThumbnailTurn(replay)
	return RenderThumbnail(replay, turnNum, DefaultThumbnailConfig())
}

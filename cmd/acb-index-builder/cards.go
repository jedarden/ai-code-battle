package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// CardConfig holds configuration for card generation
type CardConfig struct {
	Width  int
	Height int
}

// DefaultCardConfig is the default card size (1200x630 for Open Graph)
var DefaultCardConfig = CardConfig{
	Width:  1200,
	Height: 630,
}

// BotCard represents the data needed to render a bot profile card
type BotCard struct {
	BotID         string
	Name          string
	Rating        int
	WinRate       float64
	MatchesPlayed int
	Wins          int
	Losses        int
	Rank          int
	Evolved       bool
	Island        string
	Generation    int
	HealthStatus  string
}

// generateBotCard creates a PNG profile card for a bot
func generateBotCard(bot BotCard, cfg CardConfig) (*image.RGBA, error) {
	// Create image with dark background
	img := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))

	// Fill with dark background
	bgColor := color.RGBA{R: 18, G: 18, B: 24, A: 255} // #121218
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// Add gradient overlay at top
	gradientColor := color.RGBA{R: 30, G: 30, B: 45, A: 255}
	for y := 0; y < 200; y++ {
		alpha := byte(255 - (y * 255 / 200))
		overlay := color.RGBA{R: gradientColor.R, G: gradientColor.G, B: gradientColor.B, A: alpha}
		for x := 0; x < cfg.Width; x++ {
			img.Set(x, y, blendColors(img.At(x, y), overlay))
		}
	}

	// Draw accent bar at top
	accentColor := getAccentColor(bot.Evolved, bot.HealthStatus)
	for x := 0; x < cfg.Width; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, accentColor)
		}
	}

	// Draw bot name (large text)
	nameY := 100
	drawText(img, bot.Name, 60, nameY, color.RGBA{R: 255, G: 255, B: 255, A: 255}, 3.0)

	// Draw bot ID (smaller, muted)
	idY := nameY + 70
	drawText(img, "ID: "+bot.BotID, 60, idY, color.RGBA{R: 128, G: 128, B: 128, A: 255}, 1.5)

	// Draw stats in a row
	statsY := 280
	statColor := color.RGBA{R: 200, G: 200, B: 200, A: 255}
	labelColor := color.RGBA{R: 128, G: 128, B: 128, A: 255}

	// Rating
	drawText(img, fmt.Sprintf("%d", bot.Rating), 60, statsY, getColorForRating(bot.Rating), 2.5)
	drawText(img, "RATING", 60, statsY+45, labelColor, 1.0)

	// Win Rate
	winRateStr := fmt.Sprintf("%.1f%%", bot.WinRate)
	drawText(img, winRateStr, 300, statsY, getWinRateColor(bot.WinRate), 2.5)
	drawText(img, "WIN RATE", 300, statsY+45, labelColor, 1.0)

	// Matches
	drawText(img, fmt.Sprintf("%d", bot.MatchesPlayed), 540, statsY, statColor, 2.5)
	drawText(img, "MATCHES", 540, statsY+45, labelColor, 1.0)

	// W/L Record
	drawText(img, fmt.Sprintf("%dW / %dL", bot.Wins, bot.Losses), 780, statsY, statColor, 2.5)
	drawText(img, "RECORD", 780, statsY+45, labelColor, 1.0)

	// Draw rank badge if in top 100
	if bot.Rank > 0 && bot.Rank <= 100 {
		badgeX := 1000
		badgeY := 100
		badgeColor := getRankBadgeColor(bot.Rank)
		drawCircle(img, badgeX, badgeY, 50, badgeColor)
		drawText(img, fmt.Sprintf("#%d", bot.Rank), badgeX-30, badgeY+10, color.RGBA{R: 255, G: 255, B: 255, A: 255}, 1.5)
	}

	// Draw evolved badge if applicable
	if bot.Evolved {
		badgeY := 380
		evolvedColor := color.RGBA{R: 138, G: 43, B: 226, A: 255} // purple
		drawRoundedRect(img, 60, badgeY, 200, 40, 8, evolvedColor)
		evolvedText := "EVOLVED"
		if bot.Island != "" {
			evolvedText = fmt.Sprintf("EVOLVED · %s", bot.Island)
		}
		drawText(img, evolvedText, 70, badgeY+28, color.RGBA{R: 255, G: 255, B: 255, A: 255}, 1.0)
	}

	// Draw footer with branding
	footerY := cfg.Height - 50
	drawText(img, "AI Code Battle", 60, footerY, color.RGBA{R: 80, G: 80, B: 80, A: 255}, 1.2)

	return img, nil
}

// generateAllBotCards generates PNG cards for all bots and saves them to the output directory
func generateAllBotCards(data *IndexData, outputDir string) error {
	cardsDir := filepath.Join(outputDir, "cards")
	if err := os.MkdirAll(cardsDir, 0755); err != nil {
		return fmt.Errorf("create cards directory: %w", err)
	}

	cfg := DefaultCardConfig

	for i, bot := range data.Bots {
		winRate := 0.0
		losses := 0
		if bot.MatchesPlayed > 0 {
			winRate = float64(bot.MatchesWon) / float64(bot.MatchesPlayed) * 100
			losses = bot.MatchesPlayed - bot.MatchesWon
		}

		card := BotCard{
			BotID:         bot.ID,
			Name:          bot.Name,
			Rating:        int(bot.Rating),
			WinRate:       winRate,
			MatchesPlayed: bot.MatchesPlayed,
			Wins:          bot.MatchesWon,
			Losses:        losses,
			Rank:          i + 1, // Rank is position in sorted list
			Evolved:       bot.Evolved,
			Island:        bot.Island,
			Generation:    bot.Generation,
			HealthStatus:  bot.HealthStatus,
		}

		img, err := generateBotCard(card, cfg)
		if err != nil {
			slog.Error("Failed to generate bot card", "bot_id", bot.ID, "error", err)
			continue
		}

		// Save to file
		cardPath := filepath.Join(cardsDir, bot.ID+".png")
		if err := savePNG(cardPath, img); err != nil {
			slog.Error("Failed to save bot card", "bot_id", bot.ID, "error", err)
			continue
		}

		slog.Debug("Generated bot card", "bot_id", bot.ID, "path", cardPath)
	}

	slog.Info("Generated bot profile cards", "count", len(data.Bots))
	return nil
}

// uploadCardsToR2 uploads generated card images to R2 warm cache
func uploadCardsToR2(ctx context.Context, cfg *Config, outputDir string) error {
	cardsDir := filepath.Join(outputDir, "cards")

	// Check if cards directory exists
	if _, err := os.Stat(cardsDir); os.IsNotExist(err) {
		return nil // No cards to upload
	}

	// Read all card files
	entries, err := os.ReadDir(cardsDir)
	if err != nil {
		return fmt.Errorf("read cards directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".png" {
			continue
		}

		cardPath := filepath.Join(cardsDir, entry.Name())
		r2Key := "cards/" + entry.Name()

		// Upload to R2
		if err := uploadFileToR2(ctx, cfg, cardPath, r2Key); err != nil {
			slog.Error("Failed to upload card to R2", "file", entry.Name(), "error", err)
			continue
		}

		slog.Debug("Uploaded card to R2", "key", r2Key)
	}

	slog.Info("Uploaded bot cards to R2", "count", len(entries))
	return nil
}

// uploadCardsToB2 uploads generated card images to B2 cold archive
func uploadCardsToB2(ctx context.Context, cfg *Config, outputDir string) error {
	cardsDir := filepath.Join(outputDir, "cards")

	// Check if cards directory exists
	if _, err := os.Stat(cardsDir); os.IsNotExist(err) {
		return nil // No cards to upload
	}

	// Read all card files
	entries, err := os.ReadDir(cardsDir)
	if err != nil {
		return fmt.Errorf("read cards directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".png" {
			continue
		}

		cardPath := filepath.Join(cardsDir, entry.Name())
		b2Key := "cards/" + entry.Name()

		// Upload to B2
		if err := uploadFileToB2(ctx, cfg, cardPath, b2Key); err != nil {
			slog.Error("Failed to upload card to B2", "file", entry.Name(), "error", err)
			continue
		}

		slog.Debug("Uploaded card to B2", "key", b2Key)
	}

	slog.Info("Uploaded bot cards to B2", "count", len(entries))
	return nil
}

// savePNG saves an image as PNG to the specified path
func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}

// drawText draws text at the specified position using basic font
func drawText(img *image.RGBA, text string, x, y int, col color.RGBA, scale float64) {
	drawer := &font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{col},
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}

	// For larger text, we draw multiple times with offset
	if scale > 1.0 {
		// Simple scaling by drawing at multiple offsets
		steps := int(scale * 2)
		for i := 0; i < steps; i++ {
			offset := i * 6 / steps
			drawer.Dot.Y = fixed.I(y + offset)
			drawer.DrawString(text)
		}
	} else {
		drawer.DrawString(text)
	}
}

// drawCircle draws a filled circle
func drawCircle(img *image.RGBA, cx, cy, r int, col color.Color) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			dx := x - cx
			dy := y - cy
			if dx*dx+dy*dy <= r*r {
				if x >= 0 && x < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
					img.Set(x, y, col)
				}
			}
		}
	}
}

// drawRoundedRect draws a filled rounded rectangle
func drawRoundedRect(img *image.RGBA, x, y, w, h, r int, col color.Color) {
	// Draw main rectangle
	for dy := r; dy < h-r; dy++ {
		for dx := 0; dx < w; dx++ {
			px := x + dx
			py := y + dy
			if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
				img.Set(px, py, col)
			}
		}
	}

	// Draw top and bottom with rounded corners
	for dx := r; dx < w-r; dx++ {
		for _, row := range []int{0, h - 1} {
			px := x + dx
			py := y + row
			if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
				img.Set(px, py, col)
			}
		}
	}

	// Draw corner circles
	corners := []struct{ cx, cy int }{
		{x + r, y + r},
		{x + w - r, y + r},
		{x + r, y + h - r},
		{x + w - r, y + h - r},
	}
	for _, c := range corners {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx*dx+dy*dy <= r*r {
					px := c.cx + dx
					py := c.cy + dy
					if px >= 0 && px < img.Bounds().Dx() && py >= 0 && py < img.Bounds().Dy() {
						img.Set(px, py, col)
					}
				}
			}
		}
	}
}

// getAccentColor returns the accent color based on bot status
func getAccentColor(evolved bool, healthStatus string) color.RGBA {
	if evolved {
		return color.RGBA{R: 138, G: 43, B: 226, A: 255} // Purple for evolved
	}
	if healthStatus == "INACTIVE" {
		return color.RGBA{R: 128, G: 128, B: 128, A: 255} // Gray for inactive
	}
	return color.RGBA{R: 59, G: 130, B: 246, A: 255} // Blue for active
}

// getColorForRating returns a color based on rating value
func getColorForRating(rating int) color.RGBA {
	switch {
	case rating >= 2000:
		return color.RGBA{R: 255, G: 215, B: 0, A: 255} // Gold
	case rating >= 1800:
		return color.RGBA{R: 192, G: 192, B: 192, A: 255} // Silver
	case rating >= 1600:
		return color.RGBA{R: 205, G: 127, B: 50, A: 255} // Bronze
	case rating >= 1400:
		return color.RGBA{R: 100, G: 200, B: 100, A: 255} // Green
	default:
		return color.RGBA{R: 200, G: 200, B: 200, A: 255} // Light gray
	}
}

// getWinRateColor returns a color based on win rate
func getWinRateColor(winRate float64) color.RGBA {
	switch {
	case winRate >= 70:
		return color.RGBA{R: 34, G: 197, B: 94, A: 255} // Green
	case winRate >= 50:
		return color.RGBA{R: 59, G: 130, B: 246, A: 255} // Blue
	case winRate >= 30:
		return color.RGBA{R: 234, G: 179, B: 8, A: 255} // Yellow
	default:
		return color.RGBA{R: 239, G: 68, B: 68, A: 255} // Red
	}
}

// getRankBadgeColor returns a color based on rank
func getRankBadgeColor(rank int) color.RGBA {
	switch {
	case rank == 1:
		return color.RGBA{R: 255, G: 215, B: 0, A: 255} // Gold
	case rank == 2:
		return color.RGBA{R: 192, G: 192, B: 192, A: 255} // Silver
	case rank == 3:
		return color.RGBA{R: 205, G: 127, B: 50, A: 255} // Bronze
	case rank <= 10:
		return color.RGBA{R: 59, G: 130, B: 246, A: 255} // Blue
	default:
		return color.RGBA{R: 100, G: 100, B: 100, A: 255} // Gray
	}
}

// blendColors blends two colors
func blendColors(bg, fg color.Color) color.RGBA {
	br, bg2, bb, ba := bg.RGBA()
	fr, fg3, fb, fa := fg.RGBA()

	// Convert from premultiplied alpha
	if ba == 0 {
		return color.RGBA{R: uint8(fr), G: uint8(fg3), B: uint8(fb), A: uint8(fa >> 8)}
	}
	if fa == 0 {
		return color.RGBA{R: uint8(br >> 8), G: uint8(bg2 >> 8), B: uint8(bb >> 8), A: uint8(ba >> 8)}
	}

	alpha := float64(fa) / 65535.0
	r := float64(br>>8)*(1-alpha) + float64(fr>>8)*alpha
	g := float64(bg2>>8)*(1-alpha) + float64(fg3>>8)*alpha
	b := float64(bb>>8)*(1-alpha) + float64(fb>>8)*alpha
	a := float64(ba>>8)*(1-alpha) + float64(fa>>8)*alpha

	return color.RGBA{
		R: uint8(r),
		G: uint8(g),
		B: uint8(b),
		A: uint8(a),
	}
}

// uploadFileToR2 uploads a file to R2
func uploadFileToR2(ctx context.Context, cfg *Config, filePath, key string) error {
	client, err := getR2Client(cfg)
	if err != nil {
		return fmt.Errorf("create R2 client: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	contentType := getS3ContentType(key)
	if err := client.uploadFile(ctx, key, file, contentType); err != nil {
		return fmt.Errorf("upload to R2: %w", err)
	}

	slog.Debug("Uploaded file to R2", "file", filePath, "key", key)
	return nil
}

// uploadFileToB2 uploads a file to B2
func uploadFileToB2(ctx context.Context, cfg *Config, filePath, key string) error {
	client, err := getB2Client(cfg)
	if err != nil {
		return fmt.Errorf("create B2 client: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	contentType := getS3ContentType(key)
	if err := client.uploadFile(ctx, key, file, contentType); err != nil {
		return fmt.Errorf("upload to B2: %w", err)
	}

	slog.Debug("Uploaded file to B2", "file", filePath, "key", key)
	return nil
}

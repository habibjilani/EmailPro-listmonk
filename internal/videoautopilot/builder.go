package videoautopilot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// BuildConfig contains the inputs needed to generate a video and metadata.
type BuildConfig struct {
	Idea         string
	Duration     time.Duration
	OutputDir    string
	Title        string
	Description  string
	Tags         []string
	Visibility   string
	ThumbnailRef string
	Upload       bool
}

// Result captures the paths and metadata produced by BuildVideo.
type Result struct {
	VideoPath     string        `json:"videoPath"`
	ThumbnailPath string        `json:"thumbnailPath"`
	ScriptPath    string        `json:"scriptPath"`
	ReportPath    string        `json:"reportPath"`
	Metadata      VideoMetadata `json:"metadata"`
}

// VideoMetadata is written to the report and can be sent to YouTube.
type VideoMetadata struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Visibility  string   `json:"visibility"`
	Duration    string   `json:"duration"`
}

// SEOPlan contains heuristic metadata suggestions.
type SEOPlan struct {
	TitleHooks    []string `json:"titleHooks"`
	Description   string   `json:"description"`
	Hashtags      []string `json:"hashtags"`
	CallToActions []string `json:"callToActions"`
}

// BuildVideo produces a high definition placeholder video using ffmpeg along with
// a thumbnail, script, and SEO-friendly metadata. It requires ffmpeg to be
// installed on the host system.
func BuildVideo(ctx context.Context, cfg BuildConfig) (Result, error) {
	if strings.TrimSpace(cfg.Idea) == "" {
		return Result{}, errors.New("idea cannot be empty")
	}

	if cfg.Duration < 10*time.Second {
		return Result{}, fmt.Errorf("duration must be at least 10s, got %s", cfg.Duration)
	}

	if cfg.OutputDir == "" {
		return Result{}, errors.New("output directory not provided")
	}

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return Result{}, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create output dir: %w", err)
	}

	scriptPath, err := writeScript(cfg)
	if err != nil {
		return Result{}, err
	}

	slidePath, err := buildCoverSlide(cfg)
	if err != nil {
		return Result{}, err
	}

	videoPath, err := renderVideo(ctx, slidePath, cfg)
	if err != nil {
		return Result{}, err
	}

	thumbPath, err := buildThumbnail(slidePath, cfg)
	if err != nil {
		return Result{}, err
	}

	metadata := buildMetadata(cfg)

	reportPath, err := writeReport(cfg, metadata, videoPath, thumbPath, scriptPath)
	if err != nil {
		return Result{}, err
	}

	return Result{
		VideoPath:     videoPath,
		ThumbnailPath: thumbPath,
		ScriptPath:    scriptPath,
		ReportPath:    reportPath,
		Metadata:      metadata,
	}, nil
}

func writeScript(cfg BuildConfig) (string, error) {
	sections := []string{
		"Intro: Hook the viewer with a striking question or statistic.",
		"Value: Deliver the main idea in concise, visual-friendly bullets.",
		"Deep dive: Add one memorable insight and a supporting example.",
		"Call to action: Encourage likes, comments, and subscriptions.",
	}

	for len(sections) < 5 {
		sections = append(sections, fmt.Sprintf("Story beat %d: Expand on '%s'.", len(sections)+1, cfg.Idea))
	}

	durationMinutes := cfg.Duration.Minutes()
	script := fmt.Sprintf("Idea: %s\nDuration: %.2f minutes\n\nStoryboard:\n", cfg.Idea, durationMinutes)
	for idx, section := range sections {
		script += fmt.Sprintf("%d. %s\n", idx+1, section)
	}

	path := filepath.Join(cfg.OutputDir, "script.txt")
	if err := os.WriteFile(path, []byte(script), 0o644); err != nil {
		return "", fmt.Errorf("write script: %w", err)
	}

	return path, nil
}

func buildCoverSlide(cfg BuildConfig) (string, error) {
	width, height := 1920, 1080
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 18, G: 32, B: 64, A: 255}}, image.Point{}, draw.Src)

	overlay := image.NewRGBA(img.Bounds())
	draw.Draw(overlay, overlay.Bounds(), &image.Uniform{C: color.RGBA{R: 57, G: 94, B: 161, A: 200}}, image.Point{}, draw.Src)
	draw.Draw(img, img.Bounds(), overlay, image.Point{}, draw.Over)

	addText(img, 80, 200, "Video autopilot", color.RGBA{R: 255, G: 255, B: 255, A: 255})
	addText(img, 80, 320, fmt.Sprintf("Idea: %s", truncate(cfg.Idea, 48)), color.RGBA{R: 233, G: 239, B: 253, A: 255})
	addText(img, 80, 440, fmt.Sprintf("Length: %s", cfg.Duration.Truncate(time.Second)), color.RGBA{R: 200, G: 221, B: 255, A: 255})
	addText(img, 80, 560, "Built for YouTube compliance", color.RGBA{R: 255, G: 255, B: 255, A: 255})

	path := filepath.Join(cfg.OutputDir, "cover.png")
	if err := imaging.Save(img, path); err != nil {
		return "", fmt.Errorf("save cover: %w", err)
	}

	return path, nil
}

func addText(img draw.Image, x, y int, text string, col color.Color) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.Int26_6(x * 64), Y: fixed.Int26_6(y * 64)},
	}
	d.DrawString(text)
}

func truncate(val string, max int) string {
	if len(val) <= max {
		return val
	}
	return val[:max-3] + "..."
}

func renderVideo(ctx context.Context, slidePath string, cfg BuildConfig) (string, error) {
	videoPath := filepath.Join(cfg.OutputDir, "video.mp4")

	args := []string{
		"-y",
		"-loop", "1",
		"-i", slidePath,
		"-t", fmt.Sprintf("%.0f", cfg.Duration.Seconds()),
		"-vf", "scale=1920:1080,format=yuv420p",
		"-c:v", "libx264",
		videoPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg render failed: %s: %w", stderr.String(), err)
	}

	return videoPath, nil
}

func buildThumbnail(slidePath string, cfg BuildConfig) (string, error) {
	img, err := imaging.Open(slidePath)
	if err != nil {
		return "", fmt.Errorf("open slide: %w", err)
	}

	thumb := imaging.Fill(img, 1280, 720, imaging.Center, imaging.Lanczos)
	if cfg.ThumbnailRef != "" {
		addText(thumb, 40, 680, cfg.ThumbnailRef, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	}

	path := filepath.Join(cfg.OutputDir, "thumbnail.png")
	if err := imaging.Save(thumb, path); err != nil {
		return "", fmt.Errorf("save thumb: %w", err)
	}

	return path, nil
}

func buildMetadata(cfg BuildConfig) VideoMetadata {
	description := cfg.Description
	if description == "" {
		description = fmt.Sprintf("A HD walkthrough on %s. Crafted for clarity and retention.", cfg.Idea)
	}

	tags := cfg.Tags
	if len(tags) == 0 {
		tags = []string{"hd-video", "automation", "productivity", "tutorial"}
	}

	visibility := cfg.Visibility
	if visibility == "" {
		visibility = "private"
	}

	return VideoMetadata{
		Title:       defaultTitle(cfg),
		Description: description,
		Tags:        tags,
		Visibility:  visibility,
		Duration:    cfg.Duration.String(),
	}
}

func defaultTitle(cfg BuildConfig) string {
	if cfg.Title != "" {
		return cfg.Title
	}
	return fmt.Sprintf("%s | HD automation", strings.Title(cfg.Idea))
}

func writeReport(cfg BuildConfig, meta VideoMetadata, videoPath, thumbPath, scriptPath string) (string, error) {
	seo := BuildSEOPlan(meta)

	payload := map[string]any{
		"idea":            cfg.Idea,
		"metadata":        meta,
		"seoPlan":         seo,
		"videoPath":       videoPath,
		"thumbnailPath":   thumbPath,
		"scriptPath":      scriptPath,
		"uploadEnabled":   cfg.Upload,
		"visibility":      meta.Visibility,
		"generatedAt":     time.Now().UTC(),
		"youtubeGuidance": "Use OAuth2 client credentials; comply with YouTube Community Guidelines and upload limits.",
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}

	reportPath := filepath.Join(cfg.OutputDir, "report.json")
	if err := os.WriteFile(reportPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write report: %w", err)
	}

	return reportPath, nil
}

// BuildSEOPlan returns simple, policy-friendly SEO suggestions.
func BuildSEOPlan(meta VideoMetadata) SEOPlan {
	hooks := []string{
		fmt.Sprintf("%s — step-by-step in minutes", meta.Title),
		fmt.Sprintf("How to %s (HD, no fluff)", strings.ToLower(meta.Title)),
		"Zero-click summary in the first 20 seconds",
	}

	hashtags := []string{}
	for _, tag := range meta.Tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		tag = strings.ReplaceAll(tag, " ", "")
		hashtags = append(hashtags, "#"+strings.ToLower(tag))
	}

	if len(hashtags) == 0 {
		hashtags = []string{"#productivity", "#automation", "#tutorial"}
	}

	description := meta.Description + "\n\nChapters:\n00:00 Intro\n00:30 Main idea\n01:30 Deep dive\n02:30 CTA"

	ctas := []string{
		"If this helped, hit like and subscribe for more automation guides.",
		"Comment with your next idea and we will build it in an upcoming video.",
	}

	return SEOPlan{
		TitleHooks:    hooks,
		Description:   description,
		Hashtags:      hashtags,
		CallToActions: ctas,
	}
}

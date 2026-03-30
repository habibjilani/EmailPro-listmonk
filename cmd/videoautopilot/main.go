package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/knadh/listmonk/internal/videoautopilot"
)

func main() {
	idea := flag.String("idea", "", "Video idea or theme")
	duration := flag.Duration("duration", 2*time.Minute, "Length of the video (e.g. 2m, 90s)")
	output := flag.String("output", "video-output", "Directory to write generated assets")
	title := flag.String("title", "", "Custom title")
	desc := flag.String("description", "", "Video description")
	tags := flag.String("tags", "", "Comma separated tags")
	visibility := flag.String("visibility", "unlisted", "YouTube privacy status: public|unlisted|private")
	thumbnailRef := flag.String("thumb-ref", "", "Optional watermark on thumbnail")
	upload := flag.Bool("upload", false, "Attempt to upload to YouTube using YOUTUBE_ACCESS_TOKEN")
	accessToken := flag.String("youtube-token", "", "YouTube OAuth2 access token (fallback to YOUTUBE_ACCESS_TOKEN env)")

	flag.Parse()

	cfg := videoautopilot.BuildConfig{
		Idea:         *idea,
		Duration:     *duration,
		OutputDir:    *output,
		Title:        *title,
		Description:  *desc,
		Tags:         splitTags(*tags),
		Visibility:   *visibility,
		ThumbnailRef: *thumbnailRef,
		Upload:       *upload,
	}

	ctx := context.Background()
	result, err := videoautopilot.BuildVideo(ctx, cfg)
	if err != nil {
		log.Fatalf("build video: %v", err)
	}

	fmt.Printf("Video built at %s\nThumbnail: %s\nMetadata report: %s\n", result.VideoPath, result.ThumbnailPath, result.ReportPath)

	if *upload {
		uploadResult, err := videoautopilot.UploadVideo(ctx, result.Metadata, result.VideoPath, videoautopilot.UploadConfig{AccessToken: *accessToken})
		if err != nil {
			log.Fatalf("upload to youtube: %v", err)
		}
		fmt.Printf("YouTube upload complete: videoId=%s status=%s\n", uploadResult.VideoID, uploadResult.Status)
	}
}

func splitTags(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		tags = append(tags, p)
	}
	return tags
}

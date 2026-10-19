package videoautopilot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// UploadConfig holds parameters for YouTube uploads.
type UploadConfig struct {
	AccessToken string
	Audience    string
	APIBase     string
}

// UploadResult captures a subset of the YouTube response for reporting.
type UploadResult struct {
	VideoID string `json:"videoId"`
	Status  string `json:"status"`
}

// UploadVideo attempts a resumable upload to YouTube if an access token is present.
// It returns a warning-style error when credentials are missing so that the local
// pipeline remains usable in offline environments.
func UploadVideo(ctx context.Context, meta VideoMetadata, videoPath string, cfg UploadConfig) (UploadResult, error) {
	if cfg.AccessToken == "" {
		if token := os.Getenv("YOUTUBE_ACCESS_TOKEN"); token != "" {
			cfg.AccessToken = token
		}
	}

	if cfg.AccessToken == "" {
		return UploadResult{}, fmt.Errorf("youtube access token not provided; set YOUTUBE_ACCESS_TOKEN or pass UploadConfig")
	}

	apiBase := cfg.APIBase
	if apiBase == "" {
		apiBase = "https://www.googleapis.com/upload/youtube/v3"
	}

	metaPayload := map[string]any{
		"snippet": map[string]any{
			"title":       meta.Title,
			"description": meta.Description,
			"tags":        meta.Tags,
		},
		"status": map[string]any{
			"privacyStatus":           meta.Visibility,
			"selfDeclaredMadeForKids": false,
		},
	}

	body, err := json.Marshal(metaPayload)
	if err != nil {
		return UploadResult{}, fmt.Errorf("marshal metadata: %w", err)
	}

	initReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("%s/videos?uploadType=resumable&part=snippet,status", apiBase),
		bytes.NewReader(body),
	)
	if err != nil {
		return UploadResult{}, fmt.Errorf("build init request: %w", err)
	}

	initReq.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
	initReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(initReq)
	if err != nil {
		return UploadResult{}, fmt.Errorf("initiate upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return UploadResult{}, fmt.Errorf("upload init failed: %s", string(b))
	}

	uploadURL := resp.Header.Get("Location")
	if uploadURL == "" {
		return UploadResult{}, fmt.Errorf("missing resumable upload URL in response")
	}

	videoFile, err := os.Open(filepath.Clean(videoPath))
	if err != nil {
		return UploadResult{}, fmt.Errorf("open video: %w", err)
	}
	defer videoFile.Close()

	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL, videoFile)
	if err != nil {
		return UploadResult{}, fmt.Errorf("build upload request: %w", err)
	}
	uploadReq.Header.Set("Authorization", "Bearer "+cfg.AccessToken)
	uploadReq.Header.Set("Content-Type", "video/mp4")

	uploadResp, err := http.DefaultClient.Do(uploadReq)
	if err != nil {
		return UploadResult{}, fmt.Errorf("execute upload: %w", err)
	}
	defer uploadResp.Body.Close()

	if uploadResp.StatusCode < 200 || uploadResp.StatusCode >= 300 {
		b, _ := io.ReadAll(uploadResp.Body)
		return UploadResult{}, fmt.Errorf("upload failed: %s", string(b))
	}

	var result UploadResult
	if err := json.NewDecoder(uploadResp.Body).Decode(&result); err != nil {
		return UploadResult{}, fmt.Errorf("decode response: %w", err)
	}

	if result.Status == "" {
		result.Status = uploadResp.Status
	}

	return result, nil
}

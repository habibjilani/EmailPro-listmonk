# Video autopilot helper

This repository includes a lightweight helper that prepares policy-friendly, HD-ready videos and metadata for YouTube. It targets Windows but runs anywhere Go and ffmpeg are available.

## Features

- Builds a 1920x1080 MP4 from a generated cover slide for any idea and duration (>=10s).
- Writes a storyboard script, thumbnail (1280x720), metadata, and SEO recommendations.
- Optional YouTube upload using an OAuth2 access token (`YOUTUBE_ACCESS_TOKEN`).
- Generates a `report.json` summarizing outputs and compliance reminders.

## Requirements

- Go toolchain.
- ffmpeg accessible on `PATH` (used to render the HD MP4 without extra libraries).
- YouTube Data API access token if uploading. The tool uses resumable uploads and honors privacy settings.

## Usage

```powershell
# build the helper
$ go build ./cmd/videoautopilot

# generate assets without uploading
$ ./videoautopilot --idea "AI-driven productivity" --duration 2m --output C:\\temp\\video-build --visibility unlisted

# attempt an upload (requires YOUTUBE_ACCESS_TOKEN)
$ $env:YOUTUBE_ACCESS_TOKEN="ya29..."; ./videoautopilot --idea "Cloud cost savings" --duration 90s --upload
```

Outputs include:

- `video.mp4`: HD placeholder video tailored to the requested duration.
- `thumbnail.png`: YouTube-ready thumbnail with optional watermark text.
- `script.txt`: Storyboard and talking points.
- `report.json`: Metadata, SEO plan, and compliance guidance.

## Notes on YouTube compliance and SEO

- Titles and descriptions are generated to avoid clickbait and to align with YouTube Community Guidelines.
- Privacy defaults to `private` unless overridden; set `--visibility` to `public` or `unlisted` when ready.
- Tags are sanitized into hashtags inside the SEO plan. You can provide custom tags via `--tags`.
- The uploader uses resumable uploads and never attempts to bypass quotas or automations that violate YouTube policies.

## Extending for future needs

- Swap the placeholder cover slide with AI-generated scenes or footage and keep the ffmpeg render pipeline.
- Integrate a templated narration track by adding an audio input to the `ffmpeg` command in `internal/videoautopilot/builder.go`.
- Wire in periodic analytics collection by consuming the YouTube Analytics API and appending results to `report.json`.

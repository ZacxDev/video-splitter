package processor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ZacxDev/video-splitter/config"
	"github.com/ZacxDev/video-splitter/internal/ffmpeg"
	"github.com/ZacxDev/video-splitter/internal/platform"
)

// Splitter handles video splitting operations
type Splitter struct {
	opts     *config.VideoSplitterOptions
	ffmpeg   *ffmpeg.Processor
	platform platform.Platform
}

// NewSplitter creates a new video splitter
func NewSplitter(opts *config.VideoSplitterOptions) *Splitter {
	return &Splitter{
		opts:   opts,
		ffmpeg: ffmpeg.NewProcessor(opts.Verbose),
	}
}

// Templater handles video template operations
type Templater struct {
	opts   *config.VideoTemplateOptions
	ffmpeg *ffmpeg.Processor
}

// NewTemplater creates a new video templater
func NewTemplater(opts *config.VideoTemplateOptions) *Templater {
	return &Templater{
		opts:   opts,
		ffmpeg: ffmpeg.NewProcessor(opts.Verbose),
	}
}

// GetSupportedPlatforms returns a list of supported platforms
func GetSupportedPlatforms() []string {
	return platform.GetSupportedPlatforms()
}

// Helper functions
func parseSkipDuration(skip string) (float64, error) {
	if skip == "" {
		return 0, nil
	}

	duration, err := time.ParseDuration(skip)
	if err != nil {
		return 0, fmt.Errorf("invalid skip duration format: %v", err)
	}

	return duration.Seconds(), nil
}

func sanitizeFilename(filename string) string {
	sanitized := filename

	// Remove the old extension if present
	sanitized = strings.TrimSuffix(sanitized, ".mp4")
	sanitized = strings.TrimSuffix(sanitized, ".webm")

	reg := regexp.MustCompile(`[^a-zA-Z0-9-_.]`)
	sanitized = reg.ReplaceAllString(sanitized, "_")

	reg = regexp.MustCompile(`_+`)
	sanitized = reg.ReplaceAllString(sanitized, "_")

	sanitized = strings.Trim(sanitized, "_")

	return sanitized
}

func ensureOutputPath(path, format string) string {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			// Log error but continue - the actual file operation will fail if needed
			log.Printf("Warning: failed to create directory %s: %v", dir, err)
		}
	}

	// Ensure correct file extension
	ext := fmt.Sprintf(".%s", format)
	if !strings.HasSuffix(strings.ToLower(path), ext) {
		path = strings.TrimSuffix(path, filepath.Ext(path)) + ext
	}

	return path
}

package videoprocessor

import (
	"github.com/ZacxDev/video-splitter/internal/config"
	"github.com/ZacxDev/video-splitter/internal/processor"
)

// SplitVideo splits a video into chunks according to the provided options
func SplitVideo(opts *config.VideoSplitterOptions) error {
	return processor.NewSplitter(opts).Process()
}

// ApplyTemplate applies a video template to multiple input videos
func ApplyTemplate(opts *config.VideoTemplateOptions) error {
	return processor.NewTemplater(opts).Process()
}

// GetSupportedPlatforms returns a list of supported social media platforms
func GetSupportedPlatforms() []string {
	return processor.GetSupportedPlatforms()
}

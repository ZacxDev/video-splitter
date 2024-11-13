package videoprocessor

import (
	"github.com/ZacxDev/video-splitter/config"
	"github.com/ZacxDev/video-splitter/internal/platform"
	"github.com/ZacxDev/video-splitter/internal/processor"
)

// SplitVideo splits a video into chunks according to the provided options
func SplitVideo(opts *config.VideoSplitterOptions) error {
	return processor.NewSplitter(opts).Process()
}

// ApplyTemplate applies a video template to multiple input videos
func ApplyTemplate(opts *config.VideoTemplateOptions) error {
	plat, err := platform.Get(opts.TargetPlatform)
	if err != nil {
		return err
	}

	return processor.NewTemplater(opts, plat).Process()
}

// GetSupportedPlatforms returns a list of supported social media platforms
func GetSupportedPlatforms() []string {
	return processor.GetSupportedPlatforms()
}

package videoprocessor

import (
	"github.com/ZacxDev/video-splitter/config"
	"github.com/ZacxDev/video-splitter/internal/platform"
	"github.com/ZacxDev/video-splitter/internal/processor"
	"github.com/ZacxDev/video-splitter/pkg/types"
)

// SplitVideo splits a video into chunks according to the provided options
func SplitVideo(opts *config.VideoSplitterOptions) ([]types.ProcessedClip, error) {
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
func GetSupportedPlatforms() []types.ProcessingPlatform {
	return processor.GetSupportedPlatforms()
}

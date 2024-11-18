package platform

import (
	"fmt"

	"github.com/ZacxDev/video-splitter/pkg/types"
)

// Platform defines the interface for platform-specific video processing
type Platform interface {
	// GetName returns the platform name
	GetName() types.ProcessingPlatform

	// GetMaxDimensions returns the maximum allowed video dimensions
	GetMaxDimensions() (width, height int)

	// GetMaxDuration returns the maximum allowed video duration in seconds
	GetMaxDuration() int

	// GetMaxFileSize returns the maximum allowed file size in bytes
	GetMaxFileSize() int64

	// GetVideoCodec returns the preferred video codec
	GetVideoCodec() string

	// GetAudioCodec returns the preferred audio codec
	GetAudioCodec() string

	// GetVideoBitrate returns the recommended video bitrate
	GetVideoBitrate() string

	// GetAudioBitrate returns the recommended audio bitrate
	GetAudioBitrate() string

	// GetOutputFormat returns the preferred output format (e.g., "mp4", "webm")
	GetOutputFormat() string

	// ForcePortrait returns whether videos should be forced into portrait orientation
	ForcePortrait() bool
}

var platforms = make(map[types.ProcessingPlatform]Platform)

// Register adds a platform to the registry
func Register(p Platform) {
	platforms[p.GetName()] = p
}

// Get returns a platform by name
func Get(name types.ProcessingPlatform) (Platform, error) {
	p, ok := platforms[name]
	if !ok {
		return nil, fmt.Errorf("unsupported platform: %s", name)
	}
	return p, nil
}

// GetSupportedPlatforms returns a list of supported platform names
func GetSupportedPlatforms() []types.ProcessingPlatform {
	names := make([]types.ProcessingPlatform, 0, len(platforms))
	for name := range platforms {
		names = append(names, name)
	}
	return names
}

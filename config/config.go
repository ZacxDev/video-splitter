package config

import "github.com/ZacxDev/video-splitter/pkg/types"

// VideoSplitterOptions defines options for splitting videos
type VideoSplitterOptions struct {
	InputPath      string
	OutputDir      string
	ChunkDuration  int
	Skip           string
	TargetPlatform types.ProcessingPlatform
	OutputFormat   string // "mp4" or "webm"
	Verbose        bool
}

// VideoTemplateOptions defines options for applying video templates
type VideoTemplateOptions struct {
	InputPaths               []string
	OutputPath               string
	TemplateType             string
	OutputFormat             string // "mp4" or "webm"
	Verbose                  bool
	Obscurify                bool
	SkipOptimize             bool   // Skip the optimize pass when input is already properly encoded
	LandscapeBottomRightText string
	PortraitBottomRightText  string
	TargetPlatform           types.ProcessingPlatform
	OutroLines               []string
}

type VideoDimensions struct {
	Width  int
	Height int
}

const (
	// Output resolution (1280x720)
	OutputWidth  = 1280
	OutputHeight = 720

	// Template dimensions
	Template1x1Width  = OutputWidth      // 1920
	Template1x1Height = OutputHeight     // 1080
	Template2x2Width  = OutputWidth / 2  // 960
	Template2x2Height = OutputHeight / 2 // 540
	Template3x1Width  = OutputWidth / 3  // 640
	Template3x1Height = OutputHeight     // 1080

	// Target maximum file sizes (in bytes)
	Template1x1MaxSize = 50 * 1024 * 1024  // 50MB for single video
	Template2x2MaxSize = 15 * 1024 * 1024  // 15MB per quadrant
	Template3x1MaxSize = 20 * 1024 * 1024  // 20MB per third
	MaxTotalFileSize   = 700 * 1024 * 1024 // 700MB total (high-bitrate 4K sources can exceed 600MB)

	// Quality thresholds
	MinCRF = 18 // Best quality
	MaxCRF = 28 // Lowest acceptable quality

	// Temporary directory prefix
	TempDirPrefix = "video_template_"

	// Text overlay settings
	TextSize        = "36"    // Font size for bottom right text
	TextPadding     = "20"    // Padding from edges
	TextColor       = "white" // Text color
	TextBorderColor = "black" // Text border color
	TextBorderWidth = "2"     // Text border width

	OutroTextSize  = "48"    // Larger font size for outro text
	OutroDuration  = 5       // Duration in seconds for outro screen
	OutroTextColor = "white" // Text color for outro
	OutroFadeIn    = "0.5"
)

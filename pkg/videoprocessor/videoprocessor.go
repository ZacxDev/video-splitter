package videoprocessor

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

// PlatformSpec defines specifications for different social media platforms
type PlatformSpec struct {
	MaxWidth     int
	MaxHeight    int
	MaxDuration  int   // in seconds
	MaxFileSize  int64 // in bytes
	VideoCodec   string
	AudioCodec   string
	VideoBitrate string
	AudioBitrate string
}

// Platform specifications
var PlatformSpecs = map[string]PlatformSpec{
	"instagram_reel": {
		MaxWidth:     1080,
		MaxHeight:    1920,
		MaxDuration:  90,
		MaxFileSize:  250 * 1024 * 1024,
		VideoCodec:   "libvpx-vp9",
		AudioCodec:   "libopus",
		VideoBitrate: "2M",
		AudioBitrate: "128k",
	},
	"tiktok": {
		MaxWidth:     1080,
		MaxHeight:    1920,
		MaxDuration:  180,
		MaxFileSize:  287 * 1024 * 1024,
		VideoCodec:   "libvpx-vp9",
		AudioCodec:   "libopus",
		VideoBitrate: "2M",
		AudioBitrate: "128k",
	},
	"x-twitter": {
		MaxWidth:     1920,
		MaxHeight:    1200,
		MaxDuration:  140,
		MaxFileSize:  512 * 1024 * 1024,
		VideoCodec:   "libvpx-vp9",
		AudioCodec:   "libopus",
		VideoBitrate: "2M",
		AudioBitrate: "128k",
	},
	"reddit": {
		MaxWidth:     1920,
		MaxHeight:    1080,
		MaxDuration:  900,
		MaxFileSize:  1024 * 1024 * 1024,
		VideoCodec:   "libvpx-vp9",
		AudioCodec:   "libopus",
		VideoBitrate: "4M",
		AudioBitrate: "192k",
	},
}

// VideoSplitterOptions defines options for splitting videos
type VideoSplitterOptions struct {
	InputPath      string
	OutputDir      string
	ChunkDuration  int
	Skip           string
	TargetPlatform string
	Verbose        bool
	Obscurify      bool
}

// VideoMetadata contains metadata about a video file
type VideoMetadata struct {
	Duration float64
	Width    int
	Height   int
	Codec    string
}

// VideoTemplateOptions defines options for applying video templates
type VideoTemplateOptions struct {
	InputPaths      []string
	OutputPath      string
	TemplateType    string
	Verbose         bool
	Obscurify       bool
	BottomRightText string
}

// VideoDimensions represents width and height of a video
type VideoDimensions struct {
	Width  int
	Height int
}

// VideoOptimizationParams contains parameters for video optimization
type VideoOptimizationParams struct {
	Width          int
	Height         int
	Bitrate        string
	TargetFilesize int64 // in bytes
	OutputPath     string
	InputPath      string
}

// Constants for video processing
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
	Template1x1MaxSize = 30 * 1024 * 1024 // 30MB for single video
	Template2x2MaxSize = 8 * 1024 * 1024  // 8MB per quadrant
	Template3x1MaxSize = 10 * 1024 * 1024 // 10MB per third
	MaxTotalFileSize   = 50 * 1024 * 1024 // 50MB total

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
)

// GetSupportedPlatforms returns a list of supported social media platforms
func GetSupportedPlatforms() []string {
	platforms := make([]string, 0, len(PlatformSpecs))
	for platform := range PlatformSpecs {
		platforms = append(platforms, platform)
	}
	return platforms
}

// SplitVideo splits a video into chunks according to the provided options
func SplitVideo(opts *VideoSplitterOptions) error {
	if opts.Verbose {
		log.Printf("Processing input video: %s\n", opts.InputPath)
	}

	metadata, err := GetVideoMetadata(opts.InputPath)
	if err != nil {
		return fmt.Errorf("failed to get video metadata: %v", err)
	}

	if opts.Verbose {
		log.Printf("Video metadata: Duration=%.2fs, Resolution=%dx%d, Codec=%s\n",
			metadata.Duration, metadata.Width, metadata.Height, metadata.Codec)
	}

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	skipSeconds, err := parseSkipDuration(opts.Skip)
	if err != nil {
		return err
	}

	duration := metadata.Duration - skipSeconds
	if duration <= 0 {
		return fmt.Errorf("skip duration exceeds video duration")
	}

	baseFileName := filepath.Base(opts.InputPath)
	baseFileName = strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName))
	baseFileName = sanitizeFilename(baseFileName)

	if opts.Verbose {
		log.Printf("Sanitized base filename: %s\n", baseFileName)
	}

	numChunks := int(duration) / opts.ChunkDuration
	if int(duration)%opts.ChunkDuration != 0 {
		numChunks++
	}

	var platformSpec PlatformSpec
	var usePlatformSpec bool
	if opts.TargetPlatform != "" {
		spec, ok := PlatformSpecs[opts.TargetPlatform]
		if !ok {
			return fmt.Errorf("unsupported target platform: %s", opts.TargetPlatform)
		}
		platformSpec = spec
		usePlatformSpec = true

		if opts.Verbose {
			log.Printf("Using platform specifications for %s\n", opts.TargetPlatform)
		}
	}

	for i := 0; i < numChunks; i++ {
		startTime := float64(i*opts.ChunkDuration) + skipSeconds
		outputFileName := fmt.Sprintf("%s_chunk_%03d.webm", baseFileName, i+1)
		outputPath := filepath.Join(opts.OutputDir, outputFileName)

		if opts.Verbose {
			log.Printf("Processing chunk %d/%d: %s\n", i+1, numChunks, outputPath)
		}

		// Always use re-encoding for WebM output
		input := ffmpeg.Input(opts.InputPath, ffmpeg.KwArgs{
			"ss": startTime,
		})

		outputOptions := ffmpeg.KwArgs{}
		if i < numChunks-1 {
			outputOptions["t"] = opts.ChunkDuration
		}

		if usePlatformSpec {
			outputOptions["c:v"] = platformSpec.VideoCodec
			outputOptions["c:a"] = platformSpec.AudioCodec
			outputOptions["b:v"] = platformSpec.VideoBitrate
			outputOptions["b:a"] = platformSpec.AudioBitrate
		} else {
			outputOptions["c:v"] = "libvpx-vp9"
			outputOptions["c:a"] = "libopus"
			outputOptions["b:v"] = "2M"
			outputOptions["b:a"] = "128k"
		}

		// VP9-specific options
		outputOptions["speed"] = 2
		outputOptions["row-mt"] = 1
		outputOptions["tile-columns"] = 2
		outputOptions["frame-parallel"] = 1
		outputOptions["auto-alt-ref"] = 1
		outputOptions["lag-in-frames"] = 25
		outputOptions["quality"] = "good"
		outputOptions["cpu-used"] = 2

		stream := input.Output(outputPath, outputOptions)

		if opts.Verbose {
			log.Printf("FFmpeg command: %s\n", stream.String())
		}

		err := stream.OverWriteOutput().Run()
		if err != nil {
			return fmt.Errorf("error processing chunk %d: %v (FFmpeg command: %s)",
				i+1, err, stream.String())
		}

		fmt.Printf("Created chunk %d of %d: %s\n", i+1, numChunks, outputPath)
	}

	return nil
}

// GetVideoMetadata retrieves metadata about a video file
func GetVideoMetadata(inputPath string) (*VideoMetadata, error) {
	probe, err := ffmpeg.Probe(inputPath)
	if err != nil {
		return nil, fmt.Errorf("error probing video: %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(probe), &data); err != nil {
		return nil, errors.WithStack(err)
	}

	streams, ok := data["streams"].([]interface{})
	if !ok || len(streams) == 0 {
		return nil, fmt.Errorf("no streams found in video")
	}

	var videoStream map[string]interface{}
	for _, stream := range streams {
		s := stream.(map[string]interface{})
		if s["codec_type"].(string) == "video" {
			videoStream = s
			break
		}
	}

	if videoStream == nil {
		return nil, fmt.Errorf("no video stream found")
	}

	var duration float64

	// First try video stream duration
	if durationStr, ok := videoStream["duration"].(string); ok {
		if d, err := strconv.ParseFloat(strings.TrimSpace(durationStr), 64); err == nil {
			duration = d
		}
	}

	// If stream duration is not available, try format duration
	if duration == 0 {
		if format, ok := data["format"].(map[string]interface{}); ok {
			if durationStr, ok := format["duration"].(string); ok {
				if d, err := strconv.ParseFloat(strings.TrimSpace(durationStr), 64); err == nil {
					duration = d
				}
			}
		}
	}

	// If still no duration found, try calculating from frames and frame rate
	if duration == 0 {
		if nbFrames, ok := videoStream["nb_frames"].(string); ok {
			if frames, err := strconv.ParseFloat(nbFrames, 64); err == nil {
				var frameRate float64
				if rFrameRate, ok := videoStream["r_frame_rate"].(string); ok {
					if nums := strings.Split(rFrameRate, "/"); len(nums) == 2 {
						num, err1 := strconv.ParseFloat(nums[0], 64)
						den, err2 := strconv.ParseFloat(nums[1], 64)
						if err1 == nil && err2 == nil && den != 0 {
							frameRate = num / den
						}
					}
				}
				if frameRate > 0 {
					duration = frames / frameRate
				}
			}
		}
	}

	if duration == 0 {
		return nil, fmt.Errorf("could not determine video duration")
	}

	width := int(videoStream["width"].(float64))
	height := int(videoStream["height"].(float64))
	codec := videoStream["codec_name"].(string)

	return &VideoMetadata{
		Duration: duration,
		Width:    width,
		Height:   height,
		Codec:    codec,
	}, nil
}

// ApplyTemplate applies a video template to multiple input videos
func ApplyTemplate(opts *VideoTemplateOptions) error {
	if len(opts.InputPaths) == 0 {
		return fmt.Errorf("no input videos provided")
	}

	opts.OutputPath = ensureWebMExtension(opts.OutputPath)

	tempDir, err := os.MkdirTemp("", TempDirPrefix)
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	var optimizedPaths []string
	var targetDims VideoDimensions
	var targetSize int64

	switch opts.TemplateType {
	case "1x1":
		if len(opts.InputPaths) > 1 {
			log.Printf("Warning: 1x1 template only uses first video, ignoring remaining %d videos",
				len(opts.InputPaths)-1)
			opts.InputPaths = opts.InputPaths[:1]
		}
		targetDims = VideoDimensions{Width: Template1x1Width, Height: Template1x1Height}
		targetSize = Template1x1MaxSize
	case "2x2":
		if len(opts.InputPaths) > 4 {
			log.Printf("Warning: 2x2 template only uses first 4 videos, ignoring remaining %d videos",
				len(opts.InputPaths)-4)
			opts.InputPaths = opts.InputPaths[:4]
		} else if len(opts.InputPaths) < 4 {
			return fmt.Errorf("2x2 template requires exactly 4 videos, got %d", len(opts.InputPaths))
		}
		targetDims = VideoDimensions{Width: Template2x2Width, Height: Template2x2Height}
		targetSize = Template2x2MaxSize
	case "3x1":
		if len(opts.InputPaths) > 3 {
			log.Printf("Warning: 3x1 template only uses first 3 videos, ignoring remaining %d videos",
				len(opts.InputPaths)-3)
			opts.InputPaths = opts.InputPaths[:3]
		} else if len(opts.InputPaths) < 3 {
			return fmt.Errorf("3x1 template requires exactly 3 videos, got %d", len(opts.InputPaths))
		}
		targetDims = VideoDimensions{Width: Template3x1Width, Height: Template3x1Height}
		targetSize = Template3x1MaxSize
	default:
		return fmt.Errorf("unsupported template type: %s", opts.TemplateType)
	}

	for i, inputPath := range opts.InputPaths {
		// First apply obscurify effects if enabled
		processedPath := inputPath
		if opts.Obscurify {
			obscurifiedPath := filepath.Join(tempDir, ensureWebMExtension(fmt.Sprintf("obscurified_%d", i)))
			if err := applyObscurifyEffects(inputPath, obscurifiedPath, opts.Verbose); err != nil {
				return fmt.Errorf("failed to apply obscurify effects to video %s: %v", inputPath, err)
			}
			processedPath = obscurifiedPath
		}

		optimizedPath := filepath.Join(tempDir, ensureWebMExtension(fmt.Sprintf("optimized_%d", i)))
		optimizedPaths = append(optimizedPaths, optimizedPath)

		err := optimizeVideo(VideoOptimizationParams{
			Width:          targetDims.Width,
			Height:         targetDims.Height,
			TargetFilesize: targetSize,
			OutputPath:     optimizedPath,
			InputPath:      processedPath,
		}, opts.Verbose)

		if err != nil {
			return fmt.Errorf("failed to optimize video %s: %v", inputPath, err)
		}
	}

	streams := make([]*ffmpeg.Stream, len(optimizedPaths))
	for i, path := range optimizedPaths {
		streams[i] = ffmpeg.Input(path)
	}

	var output *ffmpeg.Stream
	switch opts.TemplateType {
	case "1x1":
		if len(streams) == 0 {
			return fmt.Errorf("no input streams available")
		}

		output = process1x1Template(streams[0])
	case "2x2":
		output = process2x2Template(streams)
	case "3x1":
		output = process3x1Template(streams)
	}

	if opts.BottomRightText != "" && output != nil {
		output = addBottomRightText(output, opts.BottomRightText)
	}

	if opts.Verbose {
		log.Printf("Creating final output video: %s", opts.OutputPath)
	}

	err = output.Output(opts.OutputPath, ffmpeg.KwArgs{
		// Video codec settings
		"c:v":            "libvpx-vp9", // VP9 video codec
		"b:v":            "0",          // Use constrained quality mode
		"crf":            MinCRF,       // Quality level
		"deadline":       "good",       // Encoding speed preset
		"cpu-used":       4,            // CPU usage preset (0-8, lower is higher quality)
		"row-mt":         1,            // Enable row-based multithreading
		"tile-columns":   2,            // Number of tile columns
		"frame-parallel": 1,            // Enable frame parallel decoding
		"auto-alt-ref":   1,            // Enable alternative reference frames
		"lag-in-frames":  25,           // Number of frames to look ahead
		"g":              240,          // Keyframe interval
		"keyint_min":     120,          // Minimum keyframe interval
		"pix_fmt":        "yuv420p",    // Pixel format

		// Audio codec settings
		"c:a": "libopus", // Opus audio codec
		"b:a": "128k",    // Audio bitrate
		"ac":  2,         // Audio channels
		"ar":  48000,     // Audio sample rate

		// WebM container options
		"cluster_size_limit": 2000000, // Maximum cluster size in bytes
		"cluster_time_limit": 1000,    // Maximum cluster duration in milliseconds
	}).OverWriteOutput().ErrorToStdOut().Run()

	if err != nil {
		return fmt.Errorf("failed to create final video: %v", err)
	}

	finalFileInfo, err := os.Stat(opts.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to get final file info: %v", err)
	}

	if finalFileInfo.Size() > MaxTotalFileSize {
		if opts.Verbose {
			log.Printf("Final file too large (%d bytes), re-encoding with stricter compression",
				finalFileInfo.Size())
		}

		adjustedCRF := MinCRF + 5
		err = ffmpeg.Input(opts.OutputPath).
			Output(opts.OutputPath, ffmpeg.KwArgs{
				"c:v":            "libvpx-vp9",
				"b:v":            "0",
				"crf":            adjustedCRF,
				"deadline":       "good",
				"cpu-used":       4,
				"row-mt":         1,
				"tile-columns":   2,
				"frame-parallel": 1,
				"auto-alt-ref":   1,
				"lag-in-frames":  25,
				"g":              240,
				"keyint_min":     120,
				"pix_fmt":        "yuv420p",
				"c:a":            "copy", // Copy audio stream as is

				// WebM container options
				"cluster_size_limit": 2000000,
				"cluster_time_limit": 1000,
			}).OverWriteOutput().ErrorToStdOut().Run()

		if err != nil {
			return fmt.Errorf("failed to re-encode final video: %v", err)
		}

	}

	return nil
}

// Internal helper functions

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

	reg := regexp.MustCompile(`[^a-zA-Z0-9-_.]`)
	sanitized = reg.ReplaceAllString(sanitized, "_")

	reg = regexp.MustCompile(`_+`)
	sanitized = reg.ReplaceAllString(sanitized, "_")

	sanitized = strings.Trim(sanitized, "_")

	return sanitized
}

func getOptimalThreadCount() int {
	cpuCount := runtime.NumCPU()
	// Use 75% of available cores to prevent overload
	return int(math.Max(1, float64(cpuCount)*0.75))
}

func optimizeVideo(params VideoOptimizationParams, verbose bool) error {
	params.OutputPath = ensureWebMExtension(params.OutputPath)

	if verbose {
		log.Printf("Optimizing video: %s -> %s", params.InputPath, params.OutputPath)
		log.Printf("Target dimensions: %dx%d", params.Width, params.Height)
	}

	metadata, err := GetVideoMetadata(params.InputPath)
	if err != nil {
		return fmt.Errorf("failed to get video metadata: %v", err)
	}

	optimalDims := calculateOptimalDimensions(metadata.Width, metadata.Height,
		VideoDimensions{Width: params.Width, Height: params.Height})

	currentCRF := MinCRF
	threadCount := getOptimalThreadCount()

	for attempts := 0; attempts < 3; attempts++ {
		input := ffmpeg.Input(params.InputPath)

		stream := input.Filter("scale", ffmpeg.Args{
			fmt.Sprintf("%d:%d", optimalDims.Width, optimalDims.Height),
		})

		padTop := (params.Height - optimalDims.Height) / 2
		padBottom := params.Height - optimalDims.Height - padTop
		padLeft := (params.Width - optimalDims.Width) / 2
		padRight := params.Width - optimalDims.Width - padLeft

		if padTop > 0 || padBottom > 0 || padLeft > 0 || padRight > 0 {
			stream = stream.Filter("pad", ffmpeg.Args{
				fmt.Sprintf("%d:%d:%d:%d", params.Width, params.Height, padLeft, padTop),
			}, ffmpeg.KwArgs{
				"color": "black",
			})
		}

		err = stream.Output(params.OutputPath, ffmpeg.KwArgs{
			"c:v":            "libvpx-vp9", // VP9 video codec
			"b:v":            "0",          // Use constrained quality mode
			"crf":            currentCRF,   // Quality level
			"deadline":       "good",       // Encoding speed preset
			"cpu-used":       4,            // CPU usage preset (0-8, lower is higher quality)
			"row-mt":         1,            // Enable row-based multithreading
			"tile-columns":   2,            // Number of tile columns
			"frame-parallel": 1,            // Enable frame parallel decoding
			"auto-alt-ref":   1,            // Enable alternative reference frames
			"lag-in-frames":  25,           // Number of frames to look ahead
			"threads":        threadCount,  // Limit thread count
			"g":              240,          // Keyframe interval
			"keyint_min":     120,          // Minimum keyframe interval
			"pix_fmt":        "yuv420p",    // Pixel format

			"c:a": "libopus", // Opus audio codec
			"b:a": "128k",    // Audio bitrate
			"ac":  2,         // Audio channels
			"ar":  48000,     // Audio sample rate

			// WebM container options
			"cluster_size_limit": 2000000, // Maximum cluster size in bytes
			"cluster_time_limit": 1000,    // Maximum cluster duration in milliseconds
		}).OverWriteOutput().ErrorToStdOut().Run()

		if err != nil {
			return fmt.Errorf("failed to optimize video: %v", err)
		}

		fileInfo, err := os.Stat(params.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to get optimized file info: %v", err)
		}

		if fileInfo.Size() <= params.TargetFilesize || currentCRF >= MaxCRF {
			break
		}

		currentCRF += 5
		if currentCRF > MaxCRF {
			currentCRF = MaxCRF
		}

		if verbose {
			log.Printf("File size too large (%d bytes), retrying with CRF %d",
				fileInfo.Size(), currentCRF)
		}
	}

	return nil
}

func process2x2Template(inputs []*ffmpeg.Stream) *ffmpeg.Stream {
	scaled := make([]*ffmpeg.Stream, 4)
	for i, input := range inputs {
		scaled[i] = input.Filter("scale", ffmpeg.Args{"960:540"})
	}

	topRow := ffmpeg.Filter(
		[]*ffmpeg.Stream{scaled[0], scaled[1]},
		"hstack",
		ffmpeg.Args{},
	)

	bottomRow := ffmpeg.Filter(
		[]*ffmpeg.Stream{scaled[2], scaled[3]},
		"hstack",
		ffmpeg.Args{},
	)

	return ffmpeg.Filter(
		[]*ffmpeg.Stream{topRow, bottomRow},
		"vstack",
		ffmpeg.Args{},
	)
}

func process3x1Template(inputs []*ffmpeg.Stream) *ffmpeg.Stream {
	scaled := make([]*ffmpeg.Stream, 3)
	for i, input := range inputs {
		scaled[i] = input.Filter("scale", ffmpeg.Args{"640:720"})
	}

	return ffmpeg.Filter(
		[]*ffmpeg.Stream{scaled[0], scaled[1], scaled[2]},
		"hstack",
		ffmpeg.Args{"inputs=3"},
	)
}

func calculateOptimalDimensions(srcWidth, srcHeight int, targetDims VideoDimensions) VideoDimensions {
	srcAspect := float64(srcWidth) / float64(srcHeight)
	targetAspect := float64(targetDims.Width) / float64(targetDims.Height)

	var optimalWidth, optimalHeight int

	if srcAspect > targetAspect {
		optimalWidth = targetDims.Width
		optimalHeight = int(float64(targetDims.Width) / srcAspect)

		if optimalHeight%2 != 0 {
			optimalHeight--
		}
	} else {
		optimalHeight = targetDims.Height
		optimalWidth = int(float64(targetDims.Height) * srcAspect)

		if optimalWidth%2 != 0 {
			optimalWidth--
		}
	}

	return ensureValidDimensions(VideoDimensions{
		Width:  optimalWidth,
		Height: optimalHeight,
	})
}

func ensureValidDimensions(dims VideoDimensions) VideoDimensions {
	width := dims.Width
	if width%2 != 0 {
		width--
	}

	height := dims.Height
	if height%2 != 0 {
		height--
	}

	if width < 2 {
		width = 2
	}
	if height < 2 {
		height = 2
	}

	return VideoDimensions{
		Width:  width,
		Height: height,
	}
}

func applyObscurifyEffects(inputPath, outputPath string, verbose bool) error {
	outputPath = ensureWebMExtension(outputPath)

	if verbose {
		log.Printf("Applying obscurify effects to: %s -> %s", inputPath, outputPath)
	}

	metadata, err := GetVideoMetadata(inputPath)
	if err != nil {
		return fmt.Errorf("failed to get video metadata: %v", err)
	}

	zoomScale := 1.05
	zoomWidth := int(float64(metadata.Width) * zoomScale)
	zoomHeight := int(float64(metadata.Height) * zoomScale)

	stream := ffmpeg.Input(inputPath)

	stream = stream.Filter("scale", ffmpeg.Args{
		fmt.Sprintf("%d:%d", zoomWidth, zoomHeight),
	}).Filter("crop", ffmpeg.Args{
		fmt.Sprintf("%d:%d", metadata.Width, metadata.Height),
	}).
		Filter("eq", ffmpeg.Args{
			fmt.Sprintf("gamma=%f", 1.05),
		}).Filter("vibrance", ffmpeg.Args{
		fmt.Sprintf("intensity=%f", 0.05),
	})

	err = stream.Output(outputPath, ffmpeg.KwArgs{
		"c:v":            "libvpx-vp9", // VP9 video codec
		"b:v":            "0",          // Use constrained quality mode
		"crf":            23,           // Quality level
		"deadline":       "good",       // Encoding speed preset
		"cpu-used":       4,            // CPU usage preset (0-8, lower is higher quality)
		"row-mt":         1,            // Enable row-based multithreading
		"tile-columns":   2,            // Number of tile columns
		"frame-parallel": 1,            // Enable frame parallel decoding
		"auto-alt-ref":   1,            // Enable alternative reference frames
		"lag-in-frames":  25,           // Number of frames to look ahead
		"g":              240,          // Keyframe interval
		"keyint_min":     120,          // Minimum keyframe interval
		"pix_fmt":        "yuv420p",    // Pixel format

		"c:a": "libopus", // Opus audio codec
		"b:a": "128k",    // Audio bitrate
		"ac":  2,         // Audio channels
		"ar":  48000,     // Audio sample rate

		// WebM container options
		"cluster_size_limit": 2000000, // Maximum cluster size in bytes
		"cluster_time_limit": 1000,    // Maximum cluster duration in milliseconds

		// Audio filters
		"filter:a": "asetrate=48000*1.05,volume=0.9",
		"af":       "atempo=1.1",
	}).OverWriteOutput().ErrorToStdOut().Run()

	if err != nil {
		return fmt.Errorf("failed to apply obscurify effects: %v", err)
	}

	return nil
}

func process1x1Template(input *ffmpeg.Stream) *ffmpeg.Stream {
	// Scale the video while maintaining aspect ratio
	scaled := input.Filter("scale", ffmpeg.Args{
		fmt.Sprintf("%d:%d:force_original_aspect_ratio=decrease", Template1x1Width, Template1x1Height),
	})

	// Add padding to center the video
	return scaled.Filter("pad", ffmpeg.Args{
		fmt.Sprintf("%d:%d:(ow-iw)/2:(oh-ih)/2:black", Template1x1Width, Template1x1Height),
	})
}

func addBottomRightText(input *ffmpeg.Stream, text string) *ffmpeg.Stream {
	// Escape single quotes in the text
	escapedText := strings.ReplaceAll(text, "'", "'\\''")

	// Construct the drawtext filter string without the duplicate 'drawtext='
	drawTextFilter := fmt.Sprintf(
		"text='%s':"+
			"fontsize=%s:"+
			"fontcolor=%s:"+
			"bordercolor=%s:"+
			"borderw=%s:"+
			"x=w-tw-%s:"+
			"y=h-th-%s:"+
			"shadowcolor=black:"+
			"shadowx=2:"+
			"shadowy=2:"+
			"box=1:"+
			"boxcolor=black@0.5:"+
			"boxborderw=5",
		escapedText,
		TextSize,
		TextColor,
		TextBorderColor,
		TextBorderWidth,
		TextPadding,
		TextPadding,
	)

	// Create a complex filtergraph for the text overlay
	return input.Filter("drawtext", ffmpeg.Args{drawTextFilter})
}

func ensureWebMExtension(filename string) string {
	// Remove any existing video extension
	extensions := []string{".mp4", ".webm", ".mkv", ".avi", ".mov"}
	for _, ext := range extensions {
		filename = strings.TrimSuffix(filename, ext)
	}
	return filename + ".webm"
}

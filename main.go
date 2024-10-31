package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

// Platform specifications for different social media platforms
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

var platformSpecs = map[string]PlatformSpec{
	"instagram_reel": {
		MaxWidth:     1080,
		MaxHeight:    1920,
		MaxDuration:  90,
		MaxFileSize:  250 * 1024 * 1024,
		VideoCodec:   "h264",
		AudioCodec:   "aac",
		VideoBitrate: "2M",
		AudioBitrate: "128k",
	},
	"tiktok": {
		MaxWidth:     1080,
		MaxHeight:    1920,
		MaxDuration:  180,
		MaxFileSize:  287 * 1024 * 1024,
		VideoCodec:   "h264",
		AudioCodec:   "aac",
		VideoBitrate: "2M",
		AudioBitrate: "128k",
	},
	"x-twitter": {
		MaxWidth:     1920,
		MaxHeight:    1200,
		MaxDuration:  140,
		MaxFileSize:  512 * 1024 * 1024,
		VideoCodec:   "h264",
		AudioCodec:   "aac",
		VideoBitrate: "2M",
		AudioBitrate: "128k",
	},
	"reddit": {
		MaxWidth:     1920,
		MaxHeight:    1080,
		MaxDuration:  900,
		MaxFileSize:  1024 * 1024 * 1024,
		VideoCodec:   "h264",
		AudioCodec:   "aac",
		VideoBitrate: "4M",
		AudioBitrate: "192k",
	},
}

type VideoSplitterOptions struct {
	InputPath      string
	OutputDir      string
	ChunkDuration  int
	Skip           string
	TargetPlatform string
	Verbose        bool
}

type VideoMetadata struct {
	Duration float64
	Width    int
	Height   int
	Codec    string
}

func getVideoMetadata(inputPath string) (*VideoMetadata, error) {
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

	// Find video stream
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

	// Try multiple sources for duration
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
				// Try to get frame rate
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

	// If we still don't have a duration, report an error
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
	// Remove or replace problematic characters
	sanitized := filename

	// Replace spaces and special characters with underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9-_.]`)
	sanitized = reg.ReplaceAllString(sanitized, "_")

	// Replace multiple consecutive underscores with a single one
	reg = regexp.MustCompile(`_+`)
	sanitized = reg.ReplaceAllString(sanitized, "_")

	// Trim underscores from start and end
	sanitized = strings.Trim(sanitized, "_")

	return sanitized
}

func splitVideo(opts *VideoSplitterOptions) error {
	if opts.Verbose {
		log.Printf("Processing input video: %s\n", opts.InputPath)
	}

	metadata, err := getVideoMetadata(opts.InputPath)
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
		spec, ok := platformSpecs[opts.TargetPlatform]
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
		outputFileName := fmt.Sprintf("%s_chunk_%03d.mp4", baseFileName, i+1)
		outputPath := filepath.Join(opts.OutputDir, outputFileName)

		if opts.Verbose {
			log.Printf("Processing chunk %d/%d: %s\n", i+1, numChunks, outputPath)
		}

		// Build FFmpeg command with minimal options first
		input := ffmpeg.Input(opts.InputPath, ffmpeg.KwArgs{
			"ss": startTime,
		})

		outputOptions := ffmpeg.KwArgs{}

		// Add duration limit for all chunks except the last one
		if i < numChunks-1 {
			outputOptions["t"] = opts.ChunkDuration
		}

		// Keep input codec if possible
		outputOptions["c:v"] = "copy"
		outputOptions["c:a"] = "copy"

		// Create the stream
		stream := input.Output(outputPath, outputOptions)

		if opts.Verbose {
			log.Printf("FFmpeg command: %s\n", stream.String())
		}

		// Try with simple copy first
		err := stream.OverWriteOutput().Run()

		if err != nil {
			if opts.Verbose {
				log.Printf("Simple copy failed, trying with re-encoding...")
			}

			// If copy fails, try with re-encoding
			input = ffmpeg.Input(opts.InputPath, ffmpeg.KwArgs{
				"ss": startTime,
			})

			outputOptions = ffmpeg.KwArgs{}
			if i < numChunks-1 {
				outputOptions["t"] = opts.ChunkDuration
			}

			if usePlatformSpec {
				outputOptions["c:v"] = platformSpec.VideoCodec
				outputOptions["c:a"] = platformSpec.AudioCodec
				outputOptions["b:v"] = platformSpec.VideoBitrate
				outputOptions["b:a"] = platformSpec.AudioBitrate
			} else {
				outputOptions["c:v"] = "libx264"
				outputOptions["c:a"] = "aac"
			}

			// Add essential encoding parameters
			outputOptions["preset"] = "fast"
			outputOptions["pix_fmt"] = "yuv420p"

			// Create the stream with re-encoding options
			stream = input.Output(outputPath, outputOptions)

			if opts.Verbose {
				log.Printf("Re-encode FFmpeg command: %s\n", stream.String())
			}

			err = stream.OverWriteOutput().Run()
			if err != nil {
				return fmt.Errorf("error processing chunk %d: %v (FFmpeg command: %s)",
					i+1, err, stream.String())
			}
		}

		fmt.Printf("Created chunk %d of %d: %s\n", i+1, numChunks, outputPath)
	}

	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Split command flags
	splitCmd.Flags().StringP("input", "i", "", "Input video file")
	splitCmd.Flags().StringP("output", "o", "", "Output directory")
	splitCmd.Flags().IntP("duration", "d", 15, "Duration of each chunk in seconds")
	splitCmd.Flags().StringP("skip", "s", "", "Duration to skip from start (e.g., '1s', '10s', '1m')")
	splitCmd.Flags().StringP("target-platform", "t", "",
		fmt.Sprintf("Target platform for optimization (%s)", strings.Join(getSupportedPlatforms(), ", ")))
	splitCmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging")

	// Mark required flags for split command
	splitCmd.MarkFlagRequired("input")
	splitCmd.MarkFlagRequired("output")

	// Template command flags
	templateCmd.Flags().StringP("output", "o", "", "Output video path")
	templateCmd.Flags().String("video-template", "", "Template type (2x2 or 3x1)")
	templateCmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging")

	// Mark required flags for template command
	templateCmd.MarkFlagRequired("output")
	templateCmd.MarkFlagRequired("video-template")

	// Add commands to root
	rootCmd.AddCommand(splitCmd)
	rootCmd.AddCommand(templateCmd)
}

func main() {
	// Setup logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Execute the CLI
	Execute()
}

func getSupportedPlatforms() []string {
	platforms := make([]string, 0, len(platformSpecs))
	for platform := range platformSpecs {
		platforms = append(platforms, platform)
	}
	return platforms
}

type VideoTemplateOptions struct {
	InputPaths   []string
	OutputPath   string
	TemplateType string
	Verbose      bool
}

type VideoDimensions struct {
	Width  int
	Height int
}

type VideoOptimizationParams struct {
	Width          int
	Height         int
	Bitrate        string
	TargetFilesize int64 // in bytes
	OutputPath     string
	InputPath      string
}

const (
	// Output resolution (1920x1080)
	OutputWidth  = 1920
	OutputHeight = 1080

	// 2x2 template dimensions (960x540 per video)
	Template2x2Width  = OutputWidth / 2  // 960
	Template2x2Height = OutputHeight / 2 // 540

	// 3x1 template dimensions (640x1080 per video)
	Template3x1Width  = OutputWidth / 3 // 640
	Template3x1Height = OutputHeight    // 1080

	// Target maximum file sizes (in bytes)
	Template2x2MaxSize = 8 * 1024 * 1024  // 8MB per quadrant
	Template3x1MaxSize = 10 * 1024 * 1024 // 10MB per third

	// Final output constraints
	MaxTotalFileSize = 50 * 1024 * 1024 // 50MB total

	// Quality thresholds
	MinCRF = 18 // Best quality
	MaxCRF = 28 // Lowest acceptable quality

	// Temporary directory for optimized inputs
	TempDirPrefix = "video_template_"
)

func optimizeVideo(params VideoOptimizationParams, verbose bool) error {
	if verbose {
		log.Printf("Optimizing video: %s -> %s", params.InputPath, params.OutputPath)
		log.Printf("Target dimensions: %dx%d", params.Width, params.Height)
	}

	// Get input video metadata
	metadata, err := getVideoMetadata(params.InputPath)
	if err != nil {
		return fmt.Errorf("failed to get video metadata: %v", err)
	}

	// Calculate optimal dimensions maintaining aspect ratio
	optimalDims := calculateOptimalDimensions(metadata.Width, metadata.Height,
		VideoDimensions{Width: params.Width, Height: params.Height})

	// Start with high quality and adjust if file size is too large
	currentCRF := MinCRF

	for attempts := 0; attempts < 3; attempts++ {
		input := ffmpeg.Input(params.InputPath)

		stream := input.Filter("scale", ffmpeg.Args{
			fmt.Sprintf("%d:%d", optimalDims.Width, optimalDims.Height),
		})

		// Add padding if necessary
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

		// Apply advanced encoding options
		err = stream.Output(params.OutputPath, ffmpeg.KwArgs{
			"c:v":       "libx264",
			"preset":    "veryslow", // Best compression
			"crf":       currentCRF,
			"movflags":  "+faststart",
			"pix_fmt":   "yuv420p",
			"tune":      "film",                                                          // Optimize for high visual quality
			"x264opts":  "rc-lookahead=60:ref=6:me=umh:subme=10:trellis=2:deblock=-2,-2", // High quality encoding
			"threads":   "0",
			"profile:v": "high",
			"level":     "4.1",
			// Audio optimization
			"c:a": "aac",
			"b:a": "128k",
			"ac":  "2",     // Stereo audio
			"ar":  "44100", // Standard sample rate
		}).OverWriteOutput().ErrorToStdOut().Run()

		if err != nil {
			return fmt.Errorf("failed to optimize video: %v", err)
		}

		// Check file size
		fileInfo, err := os.Stat(params.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to get optimized file info: %v", err)
		}

		if fileInfo.Size() <= params.TargetFilesize || currentCRF >= MaxCRF {
			break
		}

		// If file is too large, increase CRF and try again
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

func applyTemplate(opts *VideoTemplateOptions) error {
	if len(opts.InputPaths) == 0 {
		return fmt.Errorf("no input videos provided")
	}

	// Create temporary directory for optimized inputs
	tempDir, err := os.MkdirTemp("", TempDirPrefix)
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Validate template type and prepare optimization parameters
	var optimizedPaths []string
	var targetDims VideoDimensions
	var targetSize int64

	switch opts.TemplateType {
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

	// Optimize each input video
	for i, inputPath := range opts.InputPaths {
		optimizedPath := filepath.Join(tempDir, fmt.Sprintf("optimized_%d.mp4", i))
		optimizedPaths = append(optimizedPaths, optimizedPath)

		err := optimizeVideo(VideoOptimizationParams{
			Width:          targetDims.Width,
			Height:         targetDims.Height,
			TargetFilesize: targetSize,
			OutputPath:     optimizedPath,
			InputPath:      inputPath,
		}, opts.Verbose)

		if err != nil {
			return fmt.Errorf("failed to optimize video %s: %v", inputPath, err)
		}
	}

	// Create input streams from optimized videos
	streams := make([]*ffmpeg.Stream, len(optimizedPaths))
	for i, path := range optimizedPaths {
		streams[i] = ffmpeg.Input(path)
	}

	// Process template
	var output *ffmpeg.Stream
	switch opts.TemplateType {
	case "2x2":
		output = process2x2Template(streams)
	case "3x1":
		output = process3x1Template(streams)
	}

	if opts.Verbose {
		log.Printf("Creating final output video: %s", opts.OutputPath)
	}

	// Final encoding with maximum optimization
	err = output.Output(opts.OutputPath, ffmpeg.KwArgs{
		"c:v":       "libx264",
		"preset":    "veryslow", // Best compression
		"crf":       MinCRF,     // Start with best quality
		"movflags":  "+faststart",
		"pix_fmt":   "yuv420p",
		"tune":      "film",
		"x264opts":  "rc-lookahead=60:ref=6:me=umh:subme=10:trellis=2:deblock=-2,-2",
		"threads":   "0",
		"profile:v": "high",
		"level":     "4.1",
		// Audio optimization
		"c:a": "aac",
		"b:a": "128k",
		"ac":  "2",
		"ar":  "44100",
	}).OverWriteOutput().ErrorToStdOut().Run()

	if err != nil {
		return fmt.Errorf("failed to create final video: %v", err)
	}

	// Check final file size and re-encode if necessary
	finalFileInfo, err := os.Stat(opts.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to get final file info: %v", err)
	}

	if finalFileInfo.Size() > MaxTotalFileSize {
		if opts.Verbose {
			log.Printf("Final file too large (%d bytes), re-encoding with stricter compression",
				finalFileInfo.Size())
		}

		// Re-encode with increased CRF
		adjustedCRF := MinCRF + 5
		err = ffmpeg.Input(opts.OutputPath).
			Output(opts.OutputPath+".tmp", ffmpeg.KwArgs{
				"c:v":      "libx264",
				"preset":   "veryslow",
				"crf":      adjustedCRF,
				"movflags": "+faststart",
				"pix_fmt":  "yuv420p",
				"tune":     "film",
				"x264opts": "rc-lookahead=60:ref=6:me=umh:subme=10:trellis=2:deblock=-2,-2",
				"threads":  "0",
				// Maintain audio
				"c:a": "copy",
			}).OverWriteOutput().ErrorToStdOut().Run()

		if err != nil {
			return fmt.Errorf("failed to re-encode final video: %v", err)
		}

		// Replace original with re-encoded version
		err = os.Rename(opts.OutputPath+".tmp", opts.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to replace final video: %v", err)
		}
	}

	return nil
}

func process2x2Template(inputs []*ffmpeg.Stream) *ffmpeg.Stream {
	// Scale each video to 960x540 (half of 1920x1080)
	scaled := make([]*ffmpeg.Stream, 4)
	for i, input := range inputs {
		scaled[i] = input.Filter("scale", ffmpeg.Args{"960:540"})
	}

	// Create the 2x2 grid
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
	// Scale each video to 640x720 (one-third of 1920x1080)
	scaled := make([]*ffmpeg.Stream, 3)
	for i, input := range inputs {
		scaled[i] = input.Filter("scale", ffmpeg.Args{"640:720"})
	}

	// Stack videos horizontally
	return ffmpeg.Filter(
		[]*ffmpeg.Stream{scaled[0], scaled[1], scaled[2]},
		"hstack",
		ffmpeg.Args{"inputs=3"},
	)
}

var (
	rootCmd = &cobra.Command{
		Use:   "video-processor",
		Short: "A video processing tool for social media content",
		Long: `video-processor is a command-line tool for processing videos for social media platforms.
It supports splitting videos into chunks and arranging multiple videos in templates.

Examples:
  # Split a video into 15-second chunks
  video-processor split -i input.mp4 -o ./output --duration 15
  
  # Create a 2x2 grid from four videos
  video-processor apply-template -o output.mp4 --video-template 2x2 video1.mp4 video2.mp4 video3.mp4 video4.mp4`,
	}

	splitCmd = &cobra.Command{
		Use:   "split",
		Short: "Split a video into smaller chunks",
		Long: `Split a video file into smaller chunks with optional platform-specific optimization.
			
Supported platforms:
- instagram_reel
- tiktok
- x-twitter
- reddit

Example:
  video-processor split -i input.mp4 -o ./output -d 15 -t instagram_reel`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &VideoSplitterOptions{}

			// Get flags
			inputPath, _ := cmd.Flags().GetString("input")
			outputDir, _ := cmd.Flags().GetString("output")
			duration, _ := cmd.Flags().GetInt("duration")
			skip, _ := cmd.Flags().GetString("skip")
			targetPlatform, _ := cmd.Flags().GetString("target-platform")
			verbose, _ := cmd.Flags().GetBool("verbose")

			// Set options
			opts.InputPath = inputPath
			opts.OutputDir = outputDir
			opts.ChunkDuration = duration
			opts.Skip = skip
			opts.TargetPlatform = targetPlatform
			opts.Verbose = verbose

			// Validate input parameters
			if opts.InputPath == "" || opts.OutputDir == "" {
				return fmt.Errorf("input path and output directory are required")
			}

			return splitVideo(opts)
		},
	}

	templateCmd = &cobra.Command{
		Use:   "apply-template",
		Short: "Apply a video template to multiple input videos",
		Long: `Arrange multiple videos in predefined layouts.

Supported templates:
- 2x2: Arrange 4 videos in a 2x2 grid
- 3x1: Arrange 3 videos side by side

Example:
  video-processor apply-template -o output.mp4 --video-template 2x2 video1.mp4 video2.mp4 video3.mp4 video4.mp4`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &VideoTemplateOptions{}

			// Get input paths from args
			opts.InputPaths = args

			// Get flags
			outputPath, _ := cmd.Flags().GetString("output")
			templateType, _ := cmd.Flags().GetString("video-template")
			verbose, _ := cmd.Flags().GetBool("verbose")

			opts.OutputPath = outputPath
			opts.TemplateType = templateType
			opts.Verbose = verbose

			if opts.OutputPath == "" {
				return fmt.Errorf("output path is required")
			}

			return applyTemplate(opts)
		},
	}
)

func calculateOptimalDimensions(srcWidth, srcHeight int, targetDims VideoDimensions) VideoDimensions {
	// Calculate source and target aspect ratios
	srcAspect := float64(srcWidth) / float64(srcHeight)
	targetAspect := float64(targetDims.Width) / float64(targetDims.Height)

	var optimalWidth, optimalHeight int

	if srcAspect > targetAspect {
		// Source video is wider - fit to width
		optimalWidth = targetDims.Width
		optimalHeight = int(float64(targetDims.Width) / srcAspect)

		// Ensure height is even (required for some codecs)
		if optimalHeight%2 != 0 {
			optimalHeight--
		}
	} else {
		// Source video is taller - fit to height
		optimalHeight = targetDims.Height
		optimalWidth = int(float64(targetDims.Height) * srcAspect)

		// Ensure width is even (required for some codecs)
		if optimalWidth%2 != 0 {
			optimalWidth--
		}
	}

	// Return dimensions that maintain aspect ratio and fit within target
	return VideoDimensions{
		Width:  optimalWidth,
		Height: optimalHeight,
	}
}

// Helper function to ensure dimensions are valid for video encoding
func ensureValidDimensions(dims VideoDimensions) VideoDimensions {
	// Ensure dimensions are even (required for most codecs)
	width := dims.Width
	if width%2 != 0 {
		width--
	}

	height := dims.Height
	if height%2 != 0 {
		height--
	}

	// Ensure dimensions are at least 2px
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

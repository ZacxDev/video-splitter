package ffmpeg

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/ZacxDev/video-splitter/config"
	"github.com/ZacxDev/video-splitter/internal/platform"
	"github.com/pkg/errors"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

type CodecSettings struct {
	VideoCodec      string
	AudioCodec      string
	DefaultCRF      int
	ContainerFormat string
	FileExtension   string
	EncoderPresets  map[string]ffmpeg.KwArgs
}

var codecPresets = map[string]CodecSettings{
	"webm": {
		VideoCodec:      "libvpx-vp9",
		AudioCodec:      "libopus",
		DefaultCRF:      15,
		ContainerFormat: "webm",
		FileExtension:   ".webm",
		EncoderPresets: map[string]ffmpeg.KwArgs{
			"high_quality": {
				"quality":        "best",
				"cpu-used":       2,
				"row-mt":         1,
				"tile-columns":   2,
				"frame-parallel": 1,
				"auto-alt-ref":   1,
				"lag-in-frames":  25,
			},
		},
	},
	"mp4": {
		VideoCodec:      "libx264",
		AudioCodec:      "aac",
		DefaultCRF:      0,
		ContainerFormat: "mp4",
		FileExtension:   ".mp4",
		EncoderPresets: map[string]ffmpeg.KwArgs{
			"high_quality": {
				"preset":       "slower",
				"profile:v":    "high",
				"level":        "5.2",
				"movflags":     "+faststart",
				"bf":           3,
				"refs":         4,
				"rc-lookahead": 60,
				"x264opts":     "keyint=60:min-keyint=60:no-scenecut:rc-lookahead=60:me=tesa:merange=32:subme=11:ref=6:analyse=all:trellis=2:direct=auto:psy-rd=1.0:deblock=-1,-1:aq-mode=3:aq-strength=0.8",
				"flags":        "+cgop",
				"vsync":        2,
				"coder":        "1",
			},
		},
	},
}

func GetCodecSettings(outputFormat string) CodecSettings {
	if settings, ok := codecPresets[outputFormat]; ok {
		return settings
	}
	// Default to WebM if format not specified or invalid
	return codecPresets["webm"]
}

// VideoMetadata contains metadata about a video file
type VideoMetadata struct {
	Duration float64
	Width    int
	Height   int
	Codec    string
}

// VideoDimensions represents width and height of a video
type VideoDimensions struct {
	Width  int
	Height int
}

// Processor wraps FFmpeg functionality
type Processor struct {
	verbose bool
}

// NewProcessor creates a new FFmpeg processor
func NewProcessor(verbose bool) *Processor {
	return &Processor{
		verbose: verbose,
	}
}

// GetVideoMetadata retrieves metadata about a video file
func (p *Processor) GetVideoMetadata(inputPath string) (*VideoMetadata, error) {
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

func (p *Processor) ProcessForPlatform(inputPath, outputPath string, plat platform.Platform, startTime float64, duration int) error {
	metadata, err := p.GetVideoMetadata(inputPath)
	if err != nil {
		return fmt.Errorf("error probing video: %v", err)
	}

	// Get input bitrate
	probe, err := ffmpeg.Probe(inputPath)
	if err != nil {
		return fmt.Errorf("error probing video: %v", err)
	}

	maxWidth, maxHeight := plat.GetMaxDimensions()

	// Handle forced portrait mode
	if plat.ForcePortrait() && metadata.Width > metadata.Height {
		// For landscape videos that need to be portrait, we'll center crop
		cropWidth := (metadata.Height * 9) / 16 // Assuming 9:16 aspect ratio for portrait
		cropX := (metadata.Width - cropWidth) / 2

		// Build the filter chain - crop first, then scale
		filterComplex := fmt.Sprintf(
			"crop=%d:%d:%d:0,scale=%d:%d",
			cropWidth, metadata.Height, // crop dimensions
			cropX,               // crop position
			maxWidth, maxHeight, // final dimensions
		)

		if p.verbose {
			log.Printf("Forcing portrait mode. Cropping %dx%d from center of %dx%d video\n",
				cropWidth, metadata.Height, metadata.Width, metadata.Height)
		}

		inputBitrate, err := p.getBitrate(metadata, probe)
		if err != nil && p.verbose {
			log.Printf("Warning: Could not determine input bitrate: %v", err)
		}

		// Determine platform bitrate
		platformBitrate := extractBitrateValue(plat.GetVideoBitrate()) * 1000000 // Convert to bps
		targetBitrate := platformBitrate

		// If we have the input bitrate, use it as a ceiling
		if inputBitrate > 0 {
			maxBitrate := int64(float64(inputBitrate) * 1.05)
			if int64(targetBitrate) > maxBitrate {
				if p.verbose {
					log.Printf("Reducing target bitrate from %d to %d bps to match input",
						targetBitrate, maxBitrate)
				}
				targetBitrate = int(maxBitrate)
			}
		}

		// Convert targetBitrate to ffmpeg format
		bitrateStr := fmt.Sprintf("%dM", targetBitrate/1000000)

		inputKwargs := ffmpeg.KwArgs{
			"ss": startTime,
		}
		if duration > 0 {
			inputKwargs["t"] = duration
		}

		stream := ffmpeg.Input(inputPath, inputKwargs)

		outputKwargs := ffmpeg.KwArgs{
			"c:v":            plat.GetVideoCodec(),
			"c:a":            plat.GetAudioCodec(),
			"b:v":            bitrateStr,
			"b:a":            plat.GetAudioBitrate(),
			"filter_complex": filterComplex,
			"pix_fmt":        "yuv420p",
			"threads":        GetOptimalThreadCount(),
			"movflags":       "+faststart",
			"g":              60,
			"keyint_min":     30,
		}

		// Add codec-specific settings
		switch plat.GetVideoCodec() {
		case "libx264":
			outputKwargs["profile:v"] = "high"
			outputKwargs["level"] = "4.0"
			outputKwargs["preset"] = "slower"
			outputKwargs["x264opts"] = "no-scenecut"
			outputKwargs["maxrate"] = bitrateStr
			outputKwargs["bufsize"] = fmt.Sprintf("%dM", 2*targetBitrate/1000000)

		case "libvpx-vp9":
			outputKwargs["deadline"] = "good"
			outputKwargs["cpu-used"] = 2
			outputKwargs["row-mt"] = 1
			outputKwargs["tile-columns"] = 2
			outputKwargs["frame-parallel"] = 1
			outputKwargs["auto-alt-ref"] = 1
			outputKwargs["lag-in-frames"] = 25
		}

		err = stream.Output(outputPath, outputKwargs).
			OverWriteOutput().
			ErrorToStdOut().
			Run()

		if err != nil {
			return fmt.Errorf("failed to process video: %v", err)
		}

		return nil
	}

	// If not forcing portrait or already portrait, use existing processing logic
	return p.processNormalVideo(inputPath, outputPath, plat, startTime, duration, metadata, probe)
}

func (p *Processor) processNormalVideo(inputPath, outputPath string, plat platform.Platform, startTime float64, duration int, metadata *VideoMetadata, probe string) error {
	// Get input bitrate
	inputBitrate, err := p.getBitrate(metadata, probe)
	if err != nil && p.verbose {
		log.Printf("Warning: Could not determine input bitrate: %v", err)
	}

	maxWidth, maxHeight := plat.GetMaxDimensions()

	// First, determine if we need to rotate dimensions based on orientation
	srcIsPortrait := metadata.Height > metadata.Width
	targetIsPortrait := maxHeight > maxWidth

	if srcIsPortrait != targetIsPortrait {
		maxWidth, maxHeight = maxHeight, maxWidth
	}

	// Calculate scale dimensions while maintaining aspect ratio
	srcAspect := float64(metadata.Width) / float64(metadata.Height)
	targetAspect := float64(maxWidth) / float64(maxHeight)

	var scaleWidth, scaleHeight int
	if srcAspect > targetAspect {
		// Width limited
		scaleWidth = maxWidth
		scaleHeight = int(float64(maxWidth) / srcAspect)
	} else {
		// Height limited
		scaleHeight = maxHeight
		scaleWidth = int(float64(maxHeight) * srcAspect)
	}

	// Ensure dimensions are even
	scaleWidth = scaleWidth - (scaleWidth % 2)
	scaleHeight = scaleHeight - (scaleHeight % 2)

	// Determine platform bitrate
	platformBitrate := extractBitrateValue(plat.GetVideoBitrate()) * 1000000 // Convert to bps
	targetBitrate := platformBitrate

	// If we have the input bitrate, use it as a ceiling
	if inputBitrate > 0 {
		maxBitrate := int64(float64(inputBitrate) * 1.05)
		if int64(targetBitrate) > maxBitrate {
			if p.verbose {
				log.Printf("Reducing target bitrate from %d to %d bps to match input",
					targetBitrate, maxBitrate)
			}
			targetBitrate = int(maxBitrate)
		}
	}

	// Convert targetBitrate to ffmpeg format
	var bitrateStr string
	if targetBitrate >= 1000000 {
		bitrateStr = fmt.Sprintf("%dM", targetBitrate/1000000)
	} else {
		bitrateStr = fmt.Sprintf("%dk", targetBitrate/1000)
	}

	// Build the filter chain - scale first, then pad if needed
	var filterComplex string
	if scaleWidth == maxWidth && scaleHeight == maxHeight {
		// No padding needed if dimensions match exactly
		filterComplex = fmt.Sprintf("scale=%d:%d", scaleWidth, scaleHeight)
	} else {
		filterComplex = fmt.Sprintf(
			"scale=%d:%d,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black",
			scaleWidth, scaleHeight,
			maxWidth, maxHeight,
		)
	}

	inputKwargs := ffmpeg.KwArgs{
		"ss": startTime,
	}
	if duration > 0 {
		inputKwargs["t"] = duration
	}

	stream := ffmpeg.Input(inputPath, inputKwargs)

	outputKwargs := ffmpeg.KwArgs{
		"c:v":            plat.GetVideoCodec(),
		"c:a":            plat.GetAudioCodec(),
		"b:v":            bitrateStr,
		"b:a":            plat.GetAudioBitrate(),
		"filter_complex": filterComplex,
		"pix_fmt":        "yuv420p",
		"threads":        GetOptimalThreadCount(),
		"movflags":       "+faststart",
		"g":              60,
		"keyint_min":     30,
	}

	// Add codec-specific settings
	switch plat.GetVideoCodec() {
	case "libx264":
		outputKwargs["profile:v"] = "high"
		outputKwargs["level"] = "4.0"
		outputKwargs["preset"] = "slower"
		outputKwargs["x264opts"] = "no-scenecut"
		outputKwargs["maxrate"] = bitrateStr
		outputKwargs["bufsize"] = fmt.Sprintf("%dM", 2*targetBitrate/1000000)

	case "libvpx-vp9":
		outputKwargs["deadline"] = "good"
		outputKwargs["cpu-used"] = 2
		outputKwargs["row-mt"] = 1
		outputKwargs["tile-columns"] = 2
		outputKwargs["frame-parallel"] = 1
		outputKwargs["auto-alt-ref"] = 1
		outputKwargs["lag-in-frames"] = 25
	}

	if p.verbose {
		log.Printf("Processing video for %s platform\n", plat.GetName())
		log.Printf("Input dimensions: %dx%d (%s)\n",
			metadata.Width, metadata.Height,
			map[bool]string{true: "portrait", false: "landscape"}[metadata.Height > metadata.Width])
		log.Printf("Scale dimensions: %dx%d\n", scaleWidth, scaleHeight)
		log.Printf("Final dimensions: %dx%d\n", maxWidth, maxHeight)
		log.Printf("Input bitrate: %d bps\n", inputBitrate)
		log.Printf("Target bitrate: %d bps (%s)\n", targetBitrate, bitrateStr)
		log.Printf("Filter complex: %s\n", filterComplex)
	}

	err = stream.Output(outputPath, outputKwargs).
		OverWriteOutput().
		ErrorToStdOut().
		Run()

	if err != nil {
		return fmt.Errorf("failed to process video: %v", err)
	}

	// Log file sizes if verbose
	if p.verbose {
		outputInfo, err := os.Stat(outputPath)
		if err == nil {
			inputInfo, err := os.Stat(inputPath)
			if err == nil {
				log.Printf("Input file size: %.2f MB\n", float64(inputInfo.Size())/1024/1024)
				log.Printf("Output file size: %.2f MB\n", float64(outputInfo.Size())/1024/1024)
			}
		}
	}

	return nil
}

// Helper functions

func (p *Processor) calculateOptimalDimensions(srcWidth, srcHeight int, targetDims VideoDimensions) VideoDimensions {
	// Determine if source is portrait or landscape
	srcIsPortrait := srcHeight > srcWidth
	targetIsPortrait := targetDims.Height > targetDims.Width

	var maxWidth, maxHeight int

	// If orientations match, use target dimensions as-is
	if srcIsPortrait == targetIsPortrait {
		maxWidth = targetDims.Width
		maxHeight = targetDims.Height
	} else {
		// If orientations don't match, swap target dimensions
		maxWidth = targetDims.Height
		maxHeight = targetDims.Width
	}

	// Calculate scaling ratios
	widthRatio := float64(maxWidth) / float64(srcWidth)
	heightRatio := float64(maxHeight) / float64(srcHeight)

	// Use the smaller ratio to maintain aspect ratio
	scaleFactor := math.Min(widthRatio, heightRatio)

	// Calculate new dimensions
	optimalWidth := int(float64(srcWidth) * scaleFactor)
	optimalHeight := int(float64(srcHeight) * scaleFactor)

	// Ensure dimensions are even (required for some codecs)
	optimalWidth = optimalWidth - (optimalWidth % 2)
	optimalHeight = optimalHeight - (optimalHeight % 2)

	// Additional check to ensure we don't exceed max dimensions in either orientation
	if float64(optimalWidth) > math.Max(float64(targetDims.Width), float64(targetDims.Height)) {
		optimalWidth = int(math.Max(float64(targetDims.Width), float64(targetDims.Height)))
		optimalHeight = int(float64(optimalWidth) * float64(srcHeight) / float64(srcWidth))
		optimalHeight = optimalHeight - (optimalHeight % 2)
	}
	if float64(optimalHeight) > math.Max(float64(targetDims.Width), float64(targetDims.Height)) {
		optimalHeight = int(math.Max(float64(targetDims.Width), float64(targetDims.Height)))
		optimalWidth = int(float64(optimalHeight) * float64(srcWidth) / float64(srcHeight))
		optimalWidth = optimalWidth - (optimalWidth % 2)
	}

	return VideoDimensions{
		Width:  optimalWidth,
		Height: optimalHeight,
	}
}

func GetOptimalThreadCount() int {
	cpuCount := runtime.NumCPU()
	// Use 75% of available cores to prevent overload
	return int(math.Max(1, float64(cpuCount)*0.75))
}

func extractBitrateValue(bitrate string) int {
	// Remove the 'M' or 'k' suffix and convert to number
	value := strings.TrimRight(bitrate, "Mk")
	number, err := strconv.Atoi(value)
	if err != nil {
		return 2 // Default to 2M if parsing fails
	}

	if strings.HasSuffix(bitrate, "M") {
		return number
	} else if strings.HasSuffix(bitrate, "k") {
		return number / 1024
	}

	return number
}

func reduceBitrate(originalBitrate string) string {
	value := extractBitrateValue(originalBitrate)
	reducedValue := int(float64(value) * 0.75) // Reduce by 25%

	if strings.HasSuffix(originalBitrate, "M") {
		return fmt.Sprintf("%dM", reducedValue)
	} else if strings.HasSuffix(originalBitrate, "k") {
		return fmt.Sprintf("%dk", reducedValue)
	}

	return fmt.Sprintf("%d", reducedValue)
}

// CreateConcatFilter creates a filter for concatenating multiple video streams
func (p *Processor) CreateConcatFilter(inputs []*ffmpeg.Stream, numStreams int) *ffmpeg.Stream {
	return ffmpeg.Filter(inputs, "concat", ffmpeg.Args{
		fmt.Sprintf("n=%d", numStreams),
		"v=1",
		"a=1",
	})
}

// CreateOverlayFilter creates a filter for overlaying one video on top of another
func (p *Processor) CreateOverlayFilter(main, overlay *ffmpeg.Stream, x, y string) *ffmpeg.Stream {
	return ffmpeg.Filter([]*ffmpeg.Stream{main, overlay}, "overlay", ffmpeg.Args{
		fmt.Sprintf("x=%s", x),
		fmt.Sprintf("y=%s", y),
	})
}

// Helper function to ensure correct file extension
func EnsureExtension(filename, extension string) string {
	// Remove any existing video extension
	extensions := []string{".mp4", ".webm", ".mkv", ".avi", ".mov"}
	for _, ext := range extensions {
		filename = strings.TrimSuffix(filename, ext)
	}
	return filename + extension
}

// Helper method to retry processing with adjusted quality
func (p *Processor) getBitrate(metadata *VideoMetadata, probe string) (int64, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(probe), &data); err != nil {
		return 0, errors.WithStack(err)
	}

	// Try to get bitrate from format section first (usually more accurate)
	if format, ok := data["format"].(map[string]interface{}); ok {
		if bitrateStr, ok := format["bit_rate"].(string); ok {
			if bitrate, err := strconv.ParseInt(bitrateStr, 10, 64); err == nil {
				return bitrate, nil
			}
		}
	}

	// If format bitrate not available, try video stream
	streams, ok := data["streams"].([]interface{})
	if !ok || len(streams) == 0 {
		return 0, fmt.Errorf("no streams found")
	}

	for _, stream := range streams {
		s := stream.(map[string]interface{})
		if s["codec_type"].(string) == "video" {
			if bitrateStr, ok := s["bit_rate"].(string); ok {
				if bitrate, err := strconv.ParseInt(bitrateStr, 10, 64); err == nil {
					return bitrate, nil
				}
			}
		}
	}

	// If no explicit bitrate found, estimate from filesize and duration
	if format, ok := data["format"].(map[string]interface{}); ok {
		if sizeStr, ok := format["size"].(string); ok {
			if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
				// Convert size to bits and divide by duration
				return int64(float64(size*8) / metadata.Duration), nil
			}
		}
	}

	return 0, fmt.Errorf("could not determine bitrate")
}

func (p *Processor) OptimizeVideo(inputPath, outputPath string, targetDims config.VideoDimensions, targetSize int64, plat platform.Platform, outputFormat string) error {
	metadata, err := p.GetVideoMetadata(inputPath)
	if err != nil {
		return errors.Wrap(err, "failed to get video metadata")
	}

	maxWidth, maxHeight := plat.GetMaxDimensions()

	// First, determine if we need to rotate dimensions based on orientation
	srcIsPortrait := metadata.Height > metadata.Width
	targetIsPortrait := maxHeight > maxWidth

	if srcIsPortrait != targetIsPortrait {
		maxWidth, maxHeight = maxHeight, maxWidth
	}

	// Calculate scale dimensions while maintaining aspect ratio
	srcAspect := float64(metadata.Width) / float64(metadata.Height)
	targetAspect := float64(maxWidth) / float64(maxHeight)

	var scaleWidth, scaleHeight int
	if srcAspect > targetAspect {
		// Width limited
		scaleWidth = maxWidth
		scaleHeight = int(float64(maxWidth) / srcAspect)
	} else {
		// Height limited
		scaleHeight = maxHeight
		scaleWidth = int(float64(maxHeight) * srcAspect)
	}

	// Ensure dimensions are even
	scaleWidth = scaleWidth - (scaleWidth % 2)
	scaleHeight = scaleHeight - (scaleHeight % 2)
	// Calculate target bitrate based on size and duration

	platformBitrate := extractBitrateValue(plat.GetVideoBitrate()) * 1000000 // Convert to bps
	targetBitrate := platformBitrate

	probe, err := ffmpeg.Probe(inputPath)
	if err != nil {
		return fmt.Errorf("error probing video: %v", err)
	}

	inputBitrate, err := p.getBitrate(metadata, probe)
	if err != nil && p.verbose {
		log.Printf("Warning: Could not determine input bitrate: %v", err)
	}

	// If we have the input bitrate, use it as a ceiling
	if inputBitrate > 0 {
		maxBitrate := int64(float64(inputBitrate) * 1.05)
		if int64(targetBitrate) > maxBitrate {
			if p.verbose {
				log.Printf("Reducing target bitrate from %d to %d bps to match input",
					targetBitrate, maxBitrate)
			}
			targetBitrate = int(maxBitrate)
		}
	}

	// Convert targetBitrate to ffmpeg format
	var bitrateStr string
	if targetBitrate >= 1000000 {
		bitrateStr = fmt.Sprintf("%dM", targetBitrate/1000000)
	} else {
		bitrateStr = fmt.Sprintf("%dk", targetBitrate/1000)
	}

	// Build filter string
	var filterComplex string
	if scaleWidth == maxWidth && scaleHeight == maxHeight {
		// No padding needed if dimensions match exactly
		filterComplex = fmt.Sprintf("scale=%d:%d", scaleWidth, scaleHeight)
	} else {
		filterComplex = fmt.Sprintf(
			"scale=%d:%d,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black",
			scaleWidth, scaleHeight,
			maxWidth, maxHeight,
		)
	}

	codecSettings := GetCodecSettings(outputFormat)
	outputKwargs := ffmpeg.KwArgs{
		"c:v":            codecSettings.VideoCodec,
		"c:a":            codecSettings.AudioCodec,
		"b:v":            bitrateStr,
		"pix_fmt":        "yuv420p",
		"filter_complex": filterComplex,
		"threads":        GetOptimalThreadCount(),
		"movflags":       "+faststart",
		"g":              60,
		"keyint_min":     30,
	}

	// Apply format-specific encoder settings
	for k, v := range codecSettings.EncoderPresets["balanced"] {
		outputKwargs[k] = v
	}

	stream := ffmpeg.Input(inputPath)
	err = stream.Output(outputPath, outputKwargs).
		OverWriteOutput().
		ErrorToStdOut().
		Run()

	if err != nil {
		return errors.Wrap(err, "failed to optimize video")
	}

	return nil
}

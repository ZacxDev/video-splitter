package processor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZacxDev/video-splitter/internal/config"
	ffmpegWrap "github.com/ZacxDev/video-splitter/internal/ffmpeg"
	"github.com/pkg/errors"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

// Process handles the video template application
func (t *Templater) Process() error {
	if len(t.opts.InputPaths) == 0 {
		return fmt.Errorf("no input videos provided")
	}

	tempDir, err := os.MkdirTemp("", "video_template_")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	var targetDims config.VideoDimensions
	var targetSize int64

	// Determine template configuration
	switch t.opts.TemplateType {
	case "1x1":
		if len(t.opts.InputPaths) > 1 {
			log.Printf("Warning: 1x1 template only uses first video, ignoring remaining %d videos",
				len(t.opts.InputPaths)-1)
			t.opts.InputPaths = t.opts.InputPaths[:1]
		}
		targetDims = config.VideoDimensions{
			Width:  config.Template1x1Width,
			Height: config.Template1x1Height,
		}
		targetSize = config.Template1x1MaxSize

	case "2x2":
		if len(t.opts.InputPaths) > 4 {
			log.Printf("Warning: 2x2 template only uses first 4 videos, ignoring remaining %d videos",
				len(t.opts.InputPaths)-4)
			t.opts.InputPaths = t.opts.InputPaths[:4]
		} else if len(t.opts.InputPaths) < 4 {
			return fmt.Errorf("2x2 template requires exactly 4 videos, got %d", len(t.opts.InputPaths))
		}
		targetDims = config.VideoDimensions{
			Width:  config.Template2x2Width,
			Height: config.Template2x2Height,
		}
		targetSize = config.Template2x2MaxSize

	case "3x1":
		if len(t.opts.InputPaths) > 3 {
			log.Printf("Warning: 3x1 template only uses first 3 videos, ignoring remaining %d videos",
				len(t.opts.InputPaths)-3)
			t.opts.InputPaths = t.opts.InputPaths[:3]
		} else if len(t.opts.InputPaths) < 3 {
			return fmt.Errorf("3x1 template requires exactly 3 videos, got %d", len(t.opts.InputPaths))
		}
		targetDims = config.VideoDimensions{
			Width:  config.Template3x1Width,
			Height: config.Template3x1Height,
		}
		targetSize = config.Template3x1MaxSize

	default:
		return fmt.Errorf("unsupported template type: %s", t.opts.TemplateType)
	}

	// Prepare videos
	optimizedPaths := make([]string, 0, len(t.opts.InputPaths))
	for i, inputPath := range t.opts.InputPaths {
		processedPath := inputPath

		// Apply obscurify effects if enabled
		if t.opts.Obscurify {
			obscurifiedPath := filepath.Join(tempDir, fmt.Sprintf("obscurified_%d."+t.opts.OutputFormat, i))
			if err := t.applyObscurifyEffects(processedPath, obscurifiedPath); err != nil {
				return fmt.Errorf("failed to apply obscurify effects to video %s: %v", inputPath, err)
			}
			processedPath = obscurifiedPath
		}

		// Optimize each video
		optimizedPath := filepath.Join(tempDir, fmt.Sprintf("optimized_%d."+t.opts.OutputFormat, i))
		if err := t.optimizeVideo(processedPath, optimizedPath, targetDims, targetSize); err != nil {
			return fmt.Errorf("failed to optimize video %s: %v", inputPath, err)
		}
		optimizedPaths = append(optimizedPaths, optimizedPath)
	}

	// Create final composition
	if err := t.createComposition(optimizedPaths, t.opts.OutputPath); err != nil {
		return fmt.Errorf("failed to create final composition: %v", err)
	}

	return nil
}

func (t *Templater) createComposition(inputPaths []string, outputPath string) error {
	outputFormat := strings.ToLower(t.opts.OutputFormat)
	if outputFormat == "" {
		outputFormat = "webm"
	}
	if outputFormat != "webm" && outputFormat != "mp4" {
		return fmt.Errorf("unsupported output format: %s (supported: webm, mp4)", outputFormat)
	}

	codecSettings := ffmpegWrap.GetCodecSettings(outputFormat)

	// Prepare filter complex string based on template type
	var filterComplex string
	var numInputs int

	switch t.opts.TemplateType {
	case "1x1":
		if len(inputPaths) > 1 {
			log.Printf("Warning: 1x1 template only uses first video, ignoring remaining %d videos",
				len(inputPaths)-1)
			inputPaths = inputPaths[:1]
		}
		// Single video just needs scaling if necessary
		filterComplex = fmt.Sprintf("[0:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
			config.Template1x1Width, config.Template1x1Height,
			config.Template1x1Width, config.Template1x1Height)
		numInputs = 1

	case "2x2":
		if len(inputPaths) > 4 {
			log.Printf("Warning: 2x2 template only uses first 4 videos, ignoring remaining %d videos",
				len(inputPaths)-4)
			inputPaths = inputPaths[:4]
		} else if len(inputPaths) < 4 {
			return fmt.Errorf("2x2 template requires exactly 4 videos, got %d", len(inputPaths))
		}
		// Build 2x2 grid filter complex
		filterComplex = fmt.Sprintf(
			"[0:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2[v0];"+
				"[1:v]scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2[v1];"+
				"[2:v]scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2[v2];"+
				"[3:v]scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2[v3];"+
				"[v0][v1]hstack=inputs=2[top];"+
				"[v2][v3]hstack=inputs=2[bottom];"+
				"[top][bottom]vstack=inputs=2[v]",
			config.Template2x2Width, config.Template2x2Height)
		numInputs = 4

	case "3x1":
		if len(inputPaths) > 3 {
			log.Printf("Warning: 3x1 template only uses first 3 videos, ignoring remaining %d videos",
				len(inputPaths)-3)
			inputPaths = inputPaths[:3]
		} else if len(inputPaths) < 3 {
			return fmt.Errorf("3x1 template requires exactly 3 videos, got %d", len(inputPaths))
		}
		// Build 3x1 horizontal stack filter complex
		filterComplex = fmt.Sprintf(
			"[0:v]scale=%d:%d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2[v0];"+
				"[1:v]scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2[v1];"+
				"[2:v]scale=%[1]d:%[2]d:force_original_aspect_ratio=decrease,pad=%[1]d:%[2]d:(ow-iw)/2:(oh-ih)/2[v2];"+
				"[v0][v1][v2]hstack=inputs=3[v]",
			config.Template3x1Width, config.Template3x1Height)
		numInputs = 3
	}

	// Add text overlay if specified
	if t.opts.BottomRightText != "" {
		escapedText := strings.ReplaceAll(t.opts.BottomRightText, "'", "\\'")
		filterComplex += fmt.Sprintf(",[v]drawtext=text='%s':fontsize=36:fontcolor=white:bordercolor=black:"+
			"borderw=2:x=w-tw-20:y=h-th-20:shadowcolor=black:shadowx=2:shadowy=2:box=1:"+
			"boxcolor=black@0.5:boxborderw=5[v]", escapedText)
	}

	// Build input streams
	var inputStreams []string
	for _, path := range inputPaths[:numInputs] {
		inputStreams = append(inputStreams, "-i", path)
	}

	// Build output settings
	outputKwargs := ffmpeg.KwArgs{
		"c:v":            codecSettings.VideoCodec,
		"c:a":            codecSettings.AudioCodec,
		"filter_complex": filterComplex,
		"map":            "[v]",
		//"map":            "0:a", // Use audio from first input
		"pix_fmt": "yuv420p",
	}

	// Add format-specific encoder settings
	for k, v := range codecSettings.EncoderPresets["high_quality"] {
		outputKwargs[k] = v
	}

	// Ensure proper output extension
	outputPath = ffmpegWrap.EnsureExtension(outputPath, codecSettings.FileExtension)

	// Create the command
	args := []string{"-y"} // Overwrite output
	args = append(args, inputStreams...)

	stream := ffmpeg.Input("pipe:", ffmpeg.KwArgs{
		"format": "lavfi",
		"i":      "anullsrc",
	})

	err := stream.Output(outputPath, outputKwargs).
		OverWriteOutput().
		ErrorToStdOut().
		Run()

	if err != nil {
		return errors.Wrap(err, "failed to create composition")
	}

	if t.opts.Verbose {
		log.Printf("Successfully created composition: %s\n", outputPath)
	}

	return nil
}

func (t *Templater) optimizeVideo(inputPath, outputPath string, targetDims config.VideoDimensions, targetSize int64) error {
	outputFormat := strings.ToLower(t.opts.OutputFormat)
	if outputFormat == "" {
		outputFormat = "webm"
	}

	metadata, err := t.ffmpeg.GetVideoMetadata(inputPath)
	if err != nil {
		return errors.Wrap(err, "failed to get video metadata")
	}

	// Calculate target bitrate based on size and duration
	targetBitrate := int64(float64(targetSize*8) / metadata.Duration) // bits per second

	// Build filter string
	filterComplex := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=decrease,"+
			"pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
		targetDims.Width, targetDims.Height,
		targetDims.Width, targetDims.Height,
	)

	codecSettings := ffmpegWrap.GetCodecSettings(outputFormat)
	outputKwargs := ffmpeg.KwArgs{
		"c:v":            codecSettings.VideoCodec,
		"c:a":            codecSettings.AudioCodec,
		"b:v":            fmt.Sprintf("%d", targetBitrate),
		"maxrate":        fmt.Sprintf("%d", targetBitrate*2),
		"minrate":        fmt.Sprintf("%d", targetBitrate/2),
		"pix_fmt":        "yuv420p",
		"filter_complex": filterComplex,
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

func (t *Templater) applyObscurifyEffects(inputPath, outputPath string) error {
	outputFormat := strings.ToLower(t.opts.OutputFormat)
	if outputFormat == "" {
		outputFormat = "webm"
	}
	if outputFormat != "webm" && outputFormat != "mp4" {
		return fmt.Errorf("unsupported output format: %s (supported: webm, mp4)", outputFormat)
	}

	if t.opts.Verbose {
		log.Printf("Applying obscurify effects to %s (format: %s)\n", inputPath, outputFormat)
	}

	metadata, err := t.ffmpeg.GetVideoMetadata(inputPath)
	if err != nil {
		return errors.Wrap(err, "failed to get video metadata")
	}

	// Calculate dimensions for zoom effect
	zoomScale := 1.05
	zoomWidth := int(float64(metadata.Width) * zoomScale)
	zoomHeight := int(float64(metadata.Height) * zoomScale)

	stream := ffmpeg.Input(inputPath)

	// Apply video effects
	stream = stream.Filter("scale", ffmpeg.Args{
		fmt.Sprintf("%d:%d", zoomWidth, zoomHeight),
	}).Filter("crop", ffmpeg.Args{
		fmt.Sprintf("%d:%d", metadata.Width, metadata.Height),
	})

	// Color and visual effects
	stream = stream.Filter("eq", ffmpeg.Args{
		"gamma=1.05",
		"saturation=1.2",
		"contrast=1.1",
	})

	// Add slight blur for dreamy effect
	stream = stream.Filter("unsharp", ffmpeg.Args{
		"3:3:1.5:3:3:0.5",
	})

	// Add subtle vignette
	stream = stream.Filter("vignette", ffmpeg.Args{
		"PI/4",    // angle
		"maximum", // mode
		"0.3",     // x0
		"0.3",     // y0
		"0.8",     // factor
	})

	codecSettings := ffmpegWrap.GetCodecSettings(outputFormat)
	outputKwargs := ffmpeg.KwArgs{
		"c:v":     codecSettings.VideoCodec,
		"pix_fmt": "yuv420p",
	}

	// Format-specific settings
	switch outputFormat {
	case "webm":
		outputKwargs["c:a"] = "libopus"
		outputKwargs["b:a"] = "128k"
		outputKwargs["deadline"] = "good"
		outputKwargs["cpu-used"] = 2
		outputKwargs["row-mt"] = 1
		outputKwargs["crf"] = 30 // Slightly higher CRF for obscurify effect

		// VP9-specific settings
		outputKwargs["tile-columns"] = 2
		outputKwargs["frame-parallel"] = 1
		outputKwargs["auto-alt-ref"] = 1
		outputKwargs["lag-in-frames"] = 25

		// Audio processing for WebM
		outputKwargs["filter:a"] = fmt.Sprintf(
			"asetrate=%d,aresample=48000,atempo=%.3f",
			int(48000*1.05), // Speed up by 5%
			0.95,            // Adjust tempo to maintain pitch
		)

	case "mp4":
		outputKwargs["c:a"] = "aac"
		outputKwargs["b:a"] = "128k"
		outputKwargs["preset"] = "medium"
		outputKwargs["crf"] = 23
		outputKwargs["profile:v"] = "high"
		outputKwargs["level"] = "4.1"
		outputKwargs["movflags"] = "+faststart"

		// H.264-specific settings
		outputKwargs["x264opts"] = "no-scenecut"
		outputKwargs["maxrate"] = "4M"
		outputKwargs["bufsize"] = "8M"

		// Audio processing for MP4
		// Use the complex filter for audio effects in MP4
		outputKwargs["filter_complex"] = fmt.Sprintf(
			"[0:a]asetrate=%d,aresample=48000,atempo=%.3f[aout]",
			int(48000*1.05), // Speed up by 5%
			0.95,            // Adjust tempo to maintain pitch
		)
		outputKwargs["map"] = "[aout]"
	}

	// Add general encoding settings
	outputKwargs["threads"] = ffmpegWrap.GetOptimalThreadCount()
	outputKwargs["g"] = 240           // Keyframe interval
	outputKwargs["keyint_min"] = 120  // Minimum keyframe interval
	outputKwargs["sc_threshold"] = 40 // Scene change threshold
	outputKwargs["refs"] = 3          // Reference frames

	// Color space settings for better quality
	outputKwargs["colorspace"] = "bt709"
	outputKwargs["color_primaries"] = "bt709"
	outputKwargs["color_trc"] = "bt709"

	// Ensure correct output extension
	outputPath = ffmpegWrap.EnsureExtension(outputPath, codecSettings.FileExtension)

	if t.opts.Verbose {
		log.Printf("Applying obscurify effects with %s codec\n", codecSettings.VideoCodec)
		log.Printf("Output path: %s\n", outputPath)
	}

	err = stream.Output(outputPath, outputKwargs).
		OverWriteOutput().
		ErrorToStdOut().
		Run()

	if err != nil {
		return errors.Wrap(err, "failed to apply obscurify effects")
	}

	// Verify the output file exists and has non-zero size
	if fileInfo, err := os.Stat(outputPath); err != nil {
		return errors.Wrap(err, "failed to verify output file")
	} else if fileInfo.Size() == 0 {
		return fmt.Errorf("output file is empty: %s", outputPath)
	}

	if t.opts.Verbose {
		log.Printf("Successfully applied obscurify effects to: %s\n", outputPath)
	}

	return nil
}

func (t *Templater) addBottomRightText(input *ffmpeg.Stream, text string) *ffmpeg.Stream {
	return input.Filter("drawtext", ffmpeg.Args{
		fmt.Sprintf("text='%s':fontsize=36:fontcolor=white:bordercolor=black:borderw=2:"+
			"x=w-tw-20:y=h-th-20:shadowcolor=black:shadowx=2:shadowy=2:box=1:boxcolor=black@0.5:boxborderw=5",
			text),
	})
}

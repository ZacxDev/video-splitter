package processor

import (
	"fmt"
	"strings"

	"github.com/ZacxDev/video-splitter/config"
	ffmpegWrap "github.com/ZacxDev/video-splitter/internal/ffmpeg"
	"github.com/pkg/errors"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func (t *Templater) ApplyObscurifyEffects(inputPath, outputPath string) error {
	outputFormat := strings.ToLower(t.opts.OutputFormat)
	if outputFormat == "" {
		outputFormat = "mp4"
	}
	if outputFormat != "webm" && outputFormat != "mp4" {
		return fmt.Errorf("unsupported output format: %s (supported: webm, mp4)", outputFormat)
	}

	metadata, err := ffmpegWrap.GetVideoMetadata(inputPath)
	if err != nil {
		return errors.Wrap(err, "failed to get video metadata")
	}

	// Calculate dimensions for zoom effect
	zoomScale := 1.025
	zoomWidth := int(float64(metadata.Width) * zoomScale)
	zoomHeight := int(float64(metadata.Height) * zoomScale)

	videoFilters := []string{
		fmt.Sprintf("scale=%d:%d", zoomWidth, zoomHeight),
		fmt.Sprintf("crop=%d:%d", metadata.Width, metadata.Height),
		"eq=gamma=1.05:saturation=1.2:contrast=1.1",
		"unsharp=3:3:1.5:3:3:0.5",
		"vignette=a=0.628319:x0=w/2:y0=h/2", // PI/5 ≈ 0.628319
	}

	// Join filters with comma
	filterComplex := strings.Join(videoFilters, ",")

	// Create input stream
	stream := ffmpeg.Input(inputPath)

	codecSettings := ffmpegWrap.GetCodecSettings(outputFormat)
	outputKwargs := ffmpeg.KwArgs{
		"c:v":     codecSettings.VideoCodec,
		"pix_fmt": "yuv420p",
		"vf":      filterComplex,
	}

	// Apply format-specific encoder settings
	for k, v := range codecSettings.EncoderPresets["balanced"] {
		outputKwargs[k] = v
	}

	// Add audio effects
	audioFilter := fmt.Sprintf(
		"aresample=48000,asetrate=48000*1.05,atempo=0.95",
	)
	outputKwargs["af"] = audioFilter

	// Ensure correct output extension
	outputPath = ffmpegWrap.EnsureExtension(outputPath, codecSettings.FileExtension)

	if err := stream.Output(outputPath, outputKwargs).
		OverWriteOutput().
		ErrorToStdOut().
		Run(); err != nil {
		return errors.Wrap(err, "failed to apply obscurify effects")
	}

	return nil
}

// AddTextOverlay adds text overlay to a video
func AddTextOverlay(stream *ffmpeg.Stream, text, position string) *ffmpeg.Stream {
	// Escape single quotes in the text
	escapedText := strings.ReplaceAll(text, "'", "'\\''")

	var x, y string
	switch position {
	case "bottom-right":
		x = "w-tw-20"
		y = "h-th-20"
	case "bottom-left":
		x = "20"
		y = "h-th-20"
	case "top-right":
		x = "w-tw-20"
		y = "20"
	case "top-left":
		x = "20"
		y = "20"
	default:
		x = "w-tw-20"
		y = "h-th-20"
	}

	drawTextFilter := fmt.Sprintf(
		"text='%s':"+
			"fontsize=%s:"+
			"fontcolor=%s:"+
			"bordercolor=%s:"+
			"borderw=%s:"+
			"x=%s:"+
			"y=%s:"+
			"shadowcolor=black:"+
			"shadowx=2:"+
			"shadowy=2:"+
			"box=1:"+
			"boxcolor=black@0.5:"+
			"boxborderw=5",
		escapedText,
		config.TextSize,
		config.TextColor,
		config.TextBorderColor,
		config.TextBorderWidth,
		x,
		y,
	)

	return stream.Filter("drawtext", ffmpeg.Args{drawTextFilter})
}

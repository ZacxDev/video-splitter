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
		outputFormat = "webm"
	}
	if outputFormat != "webm" && outputFormat != "mp4" {
		return fmt.Errorf("unsupported output format: %s (supported: webm, mp4)", outputFormat)
	}

	metadata, err := t.ffmpeg.GetVideoMetadata(inputPath)
	if err != nil {
		return errors.Wrap(err, "failed to get video metadata")
	}

	// Calculate dimensions for zoom effect
	zoomScale := 1.05
	zoomWidth := int(float64(metadata.Width) * zoomScale)
	zoomHeight := int(float64(metadata.Height) * zoomScale)

	videoFilters := []string{
		fmt.Sprintf("scale=%d:%d", zoomWidth, zoomHeight),
		fmt.Sprintf("crop=%d:%d", metadata.Width, metadata.Height),
		"eq=gamma=1.05:saturation=1.2:contrast=1.1",
		"unsharp=3:3:1.5:3:3:0.5",
		"vignette=PI/4:maximum:0.3:0.3:0.8",
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
	outputKwargs["asetrate"] = fmt.Sprintf("%d", int(48000*1.05))
	outputKwargs["atempo"] = "0.95"

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

// ApplyColorEffects applies color grading effects to a video
func ApplyColorEffects(stream *ffmpeg.Stream, effectType string) *ffmpeg.Stream {
	switch effectType {
	case "warm":
		return stream.Filter("colortemperature", ffmpeg.Args{"temperature=6000"}).
			Filter("eq", ffmpeg.Args{"saturation=1.2"})
	case "cool":
		return stream.Filter("colortemperature", ffmpeg.Args{"temperature=12000"}).
			Filter("eq", ffmpeg.Args{"saturation=0.8"})
	case "vintage":
		return stream.Filter("curves", ffmpeg.Args{"preset=vintage"}).
			Filter("vignette", ffmpeg.Args{"angle=PI/4"})
	case "cinematic":
		return stream.Filter("eq", ffmpeg.Args{
			"contrast=1.1",
			"brightness=-0.05",
			"saturation=1.1",
		}).Filter("unsharp", ffmpeg.Args{"3:3:1.5"})
	default:
		return stream
	}
}

// ApplyTransition adds a transition effect between video segments
func ApplyTransition(stream *ffmpeg.Stream, transitionType string, duration float64) *ffmpeg.Stream {
	switch transitionType {
	case "fade":
		return stream.Filter("fade", ffmpeg.Args{
			"t=in:st=0:d=" + fmt.Sprint(duration),
			"t=out:st=" + fmt.Sprint(duration) + ":d=" + fmt.Sprint(duration),
		})
	case "crossfade":
		return stream.Filter("xfade", ffmpeg.Args{
			"transition=fade",
			"duration=" + fmt.Sprint(duration),
		})
	case "wipe":
		return stream.Filter("wipe", ffmpeg.Args{
			"duration=" + fmt.Sprint(duration),
		})
	default:
		return stream
	}
}

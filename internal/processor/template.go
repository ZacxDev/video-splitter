package processor

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ZacxDev/video-splitter/config"
	ffmpegWrap "github.com/ZacxDev/video-splitter/internal/ffmpeg"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

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
		// First apply obscurify effects if enabled
		processedPath := inputPath
		if t.opts.Obscurify {
			obscurifiedPath := filepath.Join(tempDir, fmt.Sprintf("obscurified_%d."+t.opts.OutputFormat, i))
			if err := t.ApplyObscurifyEffects(inputPath, obscurifiedPath); err != nil {
				return fmt.Errorf("failed to apply obscurify effects to video %s: %v", inputPath, err)
			}
			processedPath = obscurifiedPath
		}

		optimizedPath := filepath.Join(tempDir, fmt.Sprintf("optimized_%d."+t.opts.OutputFormat, i))
		optimizedPaths = append(optimizedPaths, optimizedPath)

		outputFormat := strings.ToLower(t.opts.OutputFormat)
		if outputFormat == "" {
			outputFormat = "webm"
		}

		err := t.ffmpeg.OptimizeVideo(
			processedPath,
			optimizedPath,
			targetDims,
			targetSize,
			t.platform,
			outputFormat,
		)

		if err != nil {
			return fmt.Errorf("failed to optimize video %s: %v", inputPath, err)
		}
	}

	streams := make([]*ffmpeg.Stream, len(optimizedPaths))
	for i, path := range optimizedPaths {
		streams[i] = ffmpeg.Input(path)
	}

	outputFormat := strings.ToLower(t.opts.OutputFormat)
	if outputFormat == "" {
		outputFormat = "webm"
	}

	codecSettings := ffmpegWrap.GetCodecSettings(outputFormat)

	var output *ffmpeg.Stream
	var kwargs ffmpeg.KwArgs
	switch t.opts.TemplateType {
	case "1x1":
		if len(streams) == 0 {
			return fmt.Errorf("no input streams available")
		}

		output = streams[0]
	case "2x2":
		kwargs = ffmpeg.KwArgs{
			"c:v":        codecSettings.VideoCodec,
			"c:a":        codecSettings.AudioCodec,
			"b:v":        "0",
			"pix_fmt":    "yuv420p",
			"threads":    ffmpegWrap.GetOptimalThreadCount(),
			"movflags":   "+faststart",
			"g":          60,
			"keyint_min": 30,
		}
		output = process2x2Template(streams)
	case "3x1":
		kwargs = ffmpeg.KwArgs{
			"c:v":        codecSettings.VideoCodec,
			"c:a":        codecSettings.AudioCodec,
			"b:v":        "0",
			"pix_fmt":    "yuv420p",
			"threads":    ffmpegWrap.GetOptimalThreadCount(),
			"movflags":   "+faststart",
			"g":          60,
			"keyint_min": 30,
		}
		output = process3x1Template(streams)
	}

	if t.opts.BottomRightText != "" && output != nil {
		output = t.addBottomRightText(output, t.opts.BottomRightText)
	}

	if t.opts.Verbose {
		log.Printf("Creating final output video: %s", t.opts.OutputPath)
	}

	err = output.Output(t.opts.OutputPath, kwargs).OverWriteOutput().ErrorToStdOut().Run()
	if err != nil {
		return fmt.Errorf("failed to create final video: %v", err)
	}

	finalFileInfo, err := os.Stat(t.opts.OutputPath)
	if err != nil {
		return fmt.Errorf("failed to get final file info: %v", err)
	}

	if finalFileInfo.Size() > config.MaxTotalFileSize {
		return errors.New(fmt.Sprintf("ERROR Final file too large (%d bytes)",
			finalFileInfo.Size()))
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

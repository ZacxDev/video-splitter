package processor

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	ffmpegWrap "github.com/ZacxDev/video-splitter/internal/ffmpeg"
	"github.com/ZacxDev/video-splitter/internal/platform"
)

// Process handles the video splitting operation
func (s *Splitter) Process() error {
	// If no format specified, use platform preference or default to webm
	outputFormat := strings.ToLower(s.opts.OutputFormat)
	if outputFormat == "" {
		outputFormat = "webm"
	}
	if outputFormat != "webm" && outputFormat != "mp4" {
		return fmt.Errorf("unsupported output format: %s (supported: webm, mp4)", outputFormat)
	}

	if s.opts.TargetPlatform != "" {
		plat, err := platform.Get(s.opts.TargetPlatform)
		if err != nil {
			return err
		}
		s.platform = plat
		// Override format with platform preference if none specified
		if s.opts.OutputFormat == "" {
			outputFormat = plat.GetOutputFormat()
		}
	}

	metadata, err := ffmpegWrap.GetVideoMetadata(s.opts.InputPath)
	if err != nil {
		return fmt.Errorf("failed to get video metadata: %v", err)
	}

	if s.opts.Verbose {
		log.Printf("Video metadata: Duration=%.2fs, Resolution=%dx%d, Codec=%s\n",
			metadata.Duration, metadata.Width, metadata.Height, metadata.Codec)
		log.Printf("Output format: %s\n", outputFormat)
	}

	skipSeconds, err := parseSkipDuration(s.opts.Skip)
	if err != nil {
		return err
	}

	duration := metadata.Duration - skipSeconds
	if duration <= 0 {
		return fmt.Errorf("skip duration exceeds video duration")
	}

	if err := os.MkdirAll(s.opts.OutputDir, 0755); err != nil {
		return fmt.Errorf("error creating output directory: %v", err)
	}

	baseFileName := filepath.Base(s.opts.InputPath)
	baseFileName = strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName))
	baseFileName = sanitizeFilename(baseFileName)

	numChunks := int(duration) / s.opts.ChunkDuration
	if int(duration)%s.opts.ChunkDuration != 0 {
		numChunks++
	}

	// Check platform constraints
	if s.platform != nil {
		if s.opts.ChunkDuration > s.platform.GetMaxDuration() {
			return fmt.Errorf("chunk duration %ds exceeds platform maximum of %ds",
				s.opts.ChunkDuration, s.platform.GetMaxDuration())
		}
	}

	for i := 0; i < numChunks; i++ {
		startTime := float64(i*s.opts.ChunkDuration) + skipSeconds

		extension := fmt.Sprintf(".%s", outputFormat)
		outputFileName := fmt.Sprintf("%s_chunk_%03d%s", baseFileName, i+1, extension)
		outputPath := filepath.Join(s.opts.OutputDir, outputFileName)

		if s.opts.Verbose {
			log.Printf("Processing chunk %d/%d: %s\n", i+1, numChunks, outputPath)
		}

		// Apply processing based on platform specifications
		if s.platform != nil {
			err = s.ffmpeg.ProcessForPlatform(s.opts.InputPath, outputPath, s.platform, startTime, s.opts.ChunkDuration)
		} else {
			return errors.New("platform is nil")
		}

		if err != nil {
			return fmt.Errorf("error processing chunk %d: %v", i+1, err)
		}

		if s.opts.Verbose {
			log.Printf("Completed chunk %d/%d\n", i+1, numChunks)
		}
	}

	return nil
}

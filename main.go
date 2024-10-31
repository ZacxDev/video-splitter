package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ZacxDev/video-splitter/pkg/videoprocessor"
	"github.com/spf13/cobra"
)

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
		Long: fmt.Sprintf(`Split a video file into smaller chunks with optional platform-specific optimization.

Supported platforms:
%s

Example:
  video-processor split -i input.mp4 -o ./output -d 15 -t instagram_reel`,
			formatSupportedPlatforms()),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &videoprocessor.VideoSplitterOptions{}

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

			if opts.InputPath == "" || opts.OutputDir == "" {
				return fmt.Errorf("input path and output directory are required")
			}

			return videoprocessor.SplitVideo(opts)
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
			opts := &videoprocessor.VideoTemplateOptions{}

			opts.InputPaths = args
			outputPath, _ := cmd.Flags().GetString("output")
			templateType, _ := cmd.Flags().GetString("video-template")
			verbose, _ := cmd.Flags().GetBool("verbose")

			opts.OutputPath = outputPath
			opts.TemplateType = templateType
			opts.Verbose = verbose

			if opts.OutputPath == "" {
				return fmt.Errorf("output path is required")
			}

			return videoprocessor.ApplyTemplate(opts)
		},
	}
)

func formatSupportedPlatforms() string {
	platforms := videoprocessor.GetSupportedPlatforms()
	var sb strings.Builder
	for _, platform := range platforms {
		sb.WriteString(fmt.Sprintf("- %s\n", platform))
	}
	return sb.String()
}

func init() {
	// Split command flags
	splitCmd.Flags().StringP("input", "i", "", "Input video file")
	splitCmd.Flags().StringP("output", "o", "", "Output directory")
	splitCmd.Flags().IntP("duration", "d", 15, "Duration of each chunk in seconds")
	splitCmd.Flags().StringP("skip", "s", "", "Duration to skip from start (e.g., '1s', '10s', '1m')")
	splitCmd.Flags().StringP("target-platform", "t", "",
		fmt.Sprintf("Target platform for optimization (%s)", strings.Join(videoprocessor.GetSupportedPlatforms(), ", ")))
	splitCmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging")

	splitCmd.MarkFlagRequired("input")
	splitCmd.MarkFlagRequired("output")

	// Template command flags
	templateCmd.Flags().StringP("output", "o", "", "Output video path")
	templateCmd.Flags().String("video-template", "", "Template type (2x2 or 3x1)")
	templateCmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging")

	templateCmd.MarkFlagRequired("output")
	templateCmd.MarkFlagRequired("video-template")

	rootCmd.AddCommand(splitCmd)
	rootCmd.AddCommand(templateCmd)
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/ZacxDev/video-splitter/internal/config"
	"github.com/ZacxDev/video-splitter/pkg/videoprocessor"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "video-processor",
	Short: "A video processing tool for social media content",
	Long: `video-processor is a command-line tool for processing videos for social media platforms.
It supports splitting videos into chunks and arranging multiple videos in templates.`,
}

var splitCmd = &cobra.Command{
	Use:   "split",
	Short: "Split a video into smaller chunks",
	Long: fmt.Sprintf(`Split a video file into smaller chunks with optional platform-specific optimization.

Supported platforms:
%s

Example:
  video-processor split -i input.mp4 -o ./output -d 15 -t instagram_reel`,
		formatSupportedPlatforms()),
	RunE: runSplit,
}

var templateCmd = &cobra.Command{
	Use:   "apply-template",
	Short: "Apply a video template to multiple input videos",
	Long: `Arrange multiple videos in predefined layouts.

Supported templates:
- 1x1: Single video with optional text overlay
- 2x2: Arrange 4 videos in a 2x2 grid
- 3x1: Arrange 3 videos side by side`,
	RunE: runTemplate,
}

func init() {
	// Split command flags
	splitCmd.Flags().StringP("input", "i", "", "Input video file")
	splitCmd.Flags().StringP("output", "o", "", "Output directory")
	splitCmd.Flags().IntP("duration", "d", 15, "Duration of each chunk in seconds")
	splitCmd.Flags().StringP("skip", "s", "", "Duration to skip from start (e.g., '1s', '10s', '1m')")
	splitCmd.Flags().StringP("target-platform", "t", "",
		fmt.Sprintf("Target platform for optimization (%s)",
			strings.Join(videoprocessor.GetSupportedPlatforms(), ", ")))
	splitCmd.Flags().StringP("format", "f", "webm", "Output format (webm or mp4)")
	splitCmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging")

	splitCmd.MarkFlagRequired("input")
	splitCmd.MarkFlagRequired("output")

	// Template command flags
	templateCmd.Flags().StringP("output", "o", "", "Output video path")
	templateCmd.Flags().String("video-template", "", "Template type (1x1, 2x2, or 3x1)")
	templateCmd.Flags().StringP("format", "f", "webm", "Output format (webm or mp4)")
	templateCmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging")
	templateCmd.Flags().Bool("obscurify", false, "Apply obscurify effects to input videos")
	templateCmd.Flags().String("bottom-right-text", "", "Add text overlay to bottom right of video")

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

func runSplit(cmd *cobra.Command, args []string) error {
	opts := &config.VideoSplitterOptions{}

	opts.InputPath, _ = cmd.Flags().GetString("input")
	opts.OutputDir, _ = cmd.Flags().GetString("output")
	opts.ChunkDuration, _ = cmd.Flags().GetInt("duration")
	opts.Skip, _ = cmd.Flags().GetString("skip")
	opts.TargetPlatform, _ = cmd.Flags().GetString("target-platform")
	opts.OutputFormat, _ = cmd.Flags().GetString("format")
	opts.Verbose, _ = cmd.Flags().GetBool("verbose")

	return videoprocessor.SplitVideo(opts)
}

func runTemplate(cmd *cobra.Command, args []string) error {
	opts := &config.VideoTemplateOptions{}

	opts.InputPaths = args
	opts.OutputPath, _ = cmd.Flags().GetString("output")
	opts.TemplateType, _ = cmd.Flags().GetString("video-template")
	opts.OutputFormat, _ = cmd.Flags().GetString("format")
	opts.Verbose, _ = cmd.Flags().GetBool("verbose")
	opts.Obscurify, _ = cmd.Flags().GetBool("obscurify")
	opts.BottomRightText, _ = cmd.Flags().GetString("bottom-right-text")

	return videoprocessor.ApplyTemplate(opts)
}

func formatSupportedPlatforms() string {
	platforms := videoprocessor.GetSupportedPlatforms()
	var sb strings.Builder
	for _, platform := range platforms {
		sb.WriteString(fmt.Sprintf("- %s\n", platform))
	}
	return sb.String()
}

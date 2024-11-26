package types

type ProcessingPlatform string

const (
	ProcessingPlatformInstagramReel             ProcessingPlatform = "instagram-reel"
	ProcessingPlatformReddit                    ProcessingPlatform = "reddit"
	ProcessingPlatformXTwitter                  ProcessingPlatform = "x-twitter"
	ProcessingPlatformTryonhaulcentralPortrait  ProcessingPlatform = "tryonhaulcentral-portrait"
	ProcessingPlatformTryonhaulcentralLandscape ProcessingPlatform = "tryonhaulcentral-landscape"
)

type ProcessedClip struct {
	FilePath        string
	DurationSeconds uint64
}

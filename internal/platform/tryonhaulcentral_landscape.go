package platform

import "github.com/ZacxDev/video-splitter/pkg/types"

type TryonhaulcentralLandscape struct{}

func init() {
	Register(&TryonhaulcentralLandscape{})
}

func (p *TryonhaulcentralLandscape) GetName() types.ProcessingPlatform {
	return types.ProcessingPlatformTryonhaulcentralLandscape
}

func (p *TryonhaulcentralLandscape) GetMaxDimensions() (width, height int) {
	return 1920, 1080
}

func (p *TryonhaulcentralLandscape) GetMaxDuration() int {
	return 300 // 5 minutes
}

func (p *TryonhaulcentralLandscape) GetMaxFileSize() int64 {
	return 1024 * 1024 * 1024 // 1GB
}

func (p *TryonhaulcentralLandscape) GetVideoCodec() string {
	return "libx264"
}

func (p *TryonhaulcentralLandscape) GetAudioCodec() string {
	return "aac"
}

func (p *TryonhaulcentralLandscape) GetVideoBitrate() string {
	return "4M"
}

func (p *TryonhaulcentralLandscape) GetAudioBitrate() string {
	return "192k"
}

func (p *TryonhaulcentralLandscape) GetOutputFormat() string {
	return "mp4"
}

func (p *TryonhaulcentralLandscape) ForcePortrait() bool {
	return false
}

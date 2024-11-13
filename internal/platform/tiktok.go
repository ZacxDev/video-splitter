package platform

type TikTok struct{}

func init() {
	Register(&TikTok{})
}

func (p *TikTok) GetName() string {
	return "tiktok"
}

func (p *TikTok) GetMaxDimensions() (width, height int) {
	return 1080, 1920
}

func (p *TikTok) GetMaxDuration() int {
	return 180
}

func (p *TikTok) GetMaxFileSize() int64 {
	return 287 * 1024 * 1024 // 287MB
}

func (p *TikTok) GetVideoCodec() string {
	return "libx264" // H.264 for better compatibility
}

func (p *TikTok) GetAudioCodec() string {
	return "aac"
}

func (p *TikTok) GetVideoBitrate() string {
	return "2M"
}

func (p *TikTok) GetAudioBitrate() string {
	return "128k"
}

func (p *TikTok) GetOutputFormat() string {
	return "mp4"
}

func (p *TikTok) ForcePortrait() bool {
	return true
}

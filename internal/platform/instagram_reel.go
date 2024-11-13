package platform

type Instagram struct{}

func init() {
	Register(&Instagram{})
}

func (p *Instagram) GetName() string {
	return "instagram-reel"
}

func (p *Instagram) GetMaxDimensions() (width, height int) {
	return 1080, 1920
}

func (p *Instagram) GetMaxDuration() int {
	return 90
}

func (p *Instagram) GetMaxFileSize() int64 {
	return 250 * 1024 * 1024 // 250MB
}

func (p *Instagram) GetVideoCodec() string {
	return "libx264" // H.264 for better compatibility
}

func (p *Instagram) GetAudioCodec() string {
	return "aac"
}

func (p *Instagram) GetVideoBitrate() string {
	return "2M"
}

func (p *Instagram) GetAudioBitrate() string {
	return "128k"
}

func (p *Instagram) GetOutputFormat() string {
	return "mp4"
}

func (p *Instagram) ForcePortrait() bool {
	return true
}

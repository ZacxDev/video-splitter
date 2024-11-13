package platform

type Reddit struct{}

func init() {
	Register(&Reddit{})
}

func (p *Reddit) GetName() string {
	return "reddit"
}

func (p *Reddit) GetMaxDimensions() (width, height int) {
	return 1920, 1080
}

func (p *Reddit) GetMaxDuration() int {
	return 300 // 5 minutes
}

func (p *Reddit) GetMaxFileSize() int64 {
	return 1024 * 1024 * 1024 // 1GB
}

func (p *Reddit) GetVideoCodec() string {
	return "libx264"
}

func (p *Reddit) GetAudioCodec() string {
	return "aac"
}

func (p *Reddit) GetVideoBitrate() string {
	return "4M"
}

func (p *Reddit) GetAudioBitrate() string {
	return "192k"
}

func (p *Reddit) GetOutputFormat() string {
	return "mp4"
}

func (p *Reddit) ForcePortrait() bool {
	return false
}

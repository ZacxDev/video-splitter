package platform

type Twitter struct{}

func init() {
	Register(&Twitter{})
}

func (p *Twitter) GetName() string {
	return "x-twitter"
}

func (p *Twitter) GetMaxDimensions() (width, height int) {
	return 1920, 1200
}

func (p *Twitter) GetMaxDuration() int {
	return 140
}

func (p *Twitter) GetMaxFileSize() int64 {
	return 5 * 1024 * 1024 // 5MB
}

func (p *Twitter) GetVideoCodec() string {
	return "libx264"
}

func (p *Twitter) GetAudioCodec() string {
	return "aac"
}

func (p *Twitter) GetVideoBitrate() string {
	return "2M"
}

func (p *Twitter) GetAudioBitrate() string {
	return "128k"
}

func (p *Twitter) GetOutputFormat() string {
	return "mp4"
}

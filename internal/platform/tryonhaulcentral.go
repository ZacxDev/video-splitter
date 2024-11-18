package platform

type Tryonhaulcentral struct{}

func init() {
	Register(&Tryonhaulcentral{})
}

func (p *Tryonhaulcentral) GetName() string {
	return "tryonhaulcentral"
}

func (p *Tryonhaulcentral) GetMaxDimensions() (width, height int) {
	return 1920, 1080
}

func (p *Tryonhaulcentral) GetMaxDuration() int {
	return 300 // 5 minutes
}

func (p *Tryonhaulcentral) GetMaxFileSize() int64 {
	return 1024 * 1024 * 1024 // 1GB
}

func (p *Tryonhaulcentral) GetVideoCodec() string {
	return "libx264"
}

func (p *Tryonhaulcentral) GetAudioCodec() string {
	return "aac"
}

func (p *Tryonhaulcentral) GetVideoBitrate() string {
	return "4M"
}

func (p *Tryonhaulcentral) GetAudioBitrate() string {
	return "192k"
}

func (p *Tryonhaulcentral) GetOutputFormat() string {
	return "mp4"
}

func (p *Tryonhaulcentral) ForcePortrait() bool {
	return true
}

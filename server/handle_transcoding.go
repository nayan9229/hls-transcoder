package server

import (
	"context"

	"github.com/nayan9229/hls-transcoder/transcoder"
	"github.com/rs/zerolog/log"
)

func (s *Server) ProcessTranscoding(ctx context.Context) {
	profiles := map[string]transcoder.VideoProfile{
		"1080p": {
			Width:   1080,
			Height:  1920,
			Bitrate: 5000,
			Quality: 32,
		},
		"720p": {
			Width:   720,
			Height:  1280,
			Bitrate: 2800,
			Quality: 32,
		},
		"540p": {
			Width:   540,
			Height:  960,
			Bitrate: 1800,
			Quality: 32,
		},
		"480p": {
			Width:   480,
			Height:  854,
			Bitrate: 480,
			Quality: 32,
		},
		"360p": {
			Width:   360,
			Height:  640,
			Bitrate: 800,
			Quality: 32,
		},
		"vp9_1080p": {
			Width:   1080,
			Height:  1920,
			Bitrate: 5000,
			Quality: 32,
		},
		"vp9_720p": {
			Width:   720,
			Height:  1280,
			Bitrate: 2800,
			Quality: 32,
		},
		"vp9_540p": {
			Width:   540,
			Height:  960,
			Bitrate: 1800,
			Quality: 32,
		},
		"vp9_480p": {
			Width:   480,
			Height:  854,
			Bitrate: 480,
			Quality: 32,
		},
		"vp9_360p": {
			Width:   360,
			Height:  640,
			Bitrate: 800,
			Quality: 32,
		},
	}

	transcoder := transcoder.NewTranscoder(ctx, "https://media.qa.begenuin.com/temp_video/671a1bde19e28d78ccce68c8_1730100107150.mp4", "", "", profiles)
	err := transcoder.Transcode()
	if err != nil {
		log.Error().Err(err).Msg("Error transcoding video")
	}

	//https://3e8f0ll6-z248n1u4-1pnite53jjbk.ac2-preview.marscode.dev/video/-dHGJYlu48HGIIJ8XYItu/master.m3u8
}

package main

import (
	"context"

	"github.com/nayan9229/hls-transcoder/transcoder"
	"github.com/rs/zerolog/log"
)

func main() {
	// str, err := os.Getwd()

	// fmt.Printf("str: %T, %v\n", str, str)
	// fmt.Printf("err: %T, %v\n", err, err)

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	transcoder := transcoder.NewTranscoder(ctx, "https://media.begenuin.com/temp_video/66ab4df2161873b2f738933d_1722943171245.mp4", "", "", profiles)
	err := transcoder.Transcode()
	if err != nil {
		log.Error().Err(err).Msg("Error transcoding video")
	}
}

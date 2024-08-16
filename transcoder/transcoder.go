package transcoder

import (
	"context"
	"fmt"
	slog "log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/rs/zerolog/log"
)

type VideoProfile struct {
	Width   int
	Height  int
	Bitrate int // in kilobytes
	Quality int
}

type AudioProfile struct {
	Bitrate int // in kilobytes
}

type Transcoder struct {
	TrascodingID   string
	MediaDir       string
	MediaUrl       string
	TranscodeDir   string
	VideoProfiles  map[string]VideoProfile
	VideoKeyframes bool
	AudioProfile   AudioProfile
	Cache          bool
	CacheDir       string
	FFmpegBinary   string
	FFprobeBinary  string
	playlist       string    // m3u8 playlist string
	breakpoints    []float64 // list of breakpoints for segments
	ctx            context.Context
	cancel         context.CancelFunc

	mu sync.Mutex

	segmentLength    float64
	segmentOffset    float64
	segmentBufferMin int    // minimum segments available after playing head
	segmentBufferMax int    // maximum segments to be transcoded at once
	segmentPrefix    string // e.g. prefix-000001.ts

	metadata *ProbeMediaData
}

func NewTranscoder(ctx context.Context, mediaUrl, fFmpeg, fFprobe string, videoProfiles map[string]VideoProfile) *Transcoder {
	if len(videoProfiles) == 0 {
		log.Fatal().Msg("specify at least one video profile")
		return nil
	}
	id, err := gonanoid.New()
	if err != nil {
		log.Error().Err(err).Msg("Error generating id")
		return nil
	}

	baseDir := ""
	cwd, err := os.Getwd()
	if err != nil {
		baseDir = "/etc/transcode"
		log.Error().Err(err).Msg("Error getting current working directory")
	} else {
		baseDir = cwd
	}

	transcoder := Transcoder{
		TrascodingID:   id,
		MediaUrl:       mediaUrl,
		TranscodeDir:   fmt.Sprintf("%s/output/%s", baseDir, id),
		VideoProfiles:  videoProfiles,
		FFmpegBinary:   fFmpeg,
		FFprobeBinary:  fFprobe,
		VideoKeyframes: true,

		segmentLength:    1,
		segmentOffset:    1,
		segmentBufferMin: 1,
		segmentBufferMax: 2,
		segmentPrefix:    "chunk",

		ctx: ctx,
	}

	if _, err := os.Stat(transcoder.TranscodeDir); os.IsNotExist(err) {
		err = os.Mkdir(transcoder.TranscodeDir, 0755)
		if err != nil {
			log.Error().Err(err).Msg("Error creating transcode directory")
		}
	}

	if transcoder.FFmpegBinary == "" {
		// transcoder.FFmpegBinary = fmt.Sprintf("%s/utils/ffmpeg", baseDir)
		transcoder.FFmpegBinary = "ffmpeg"
	}

	if transcoder.FFprobeBinary == "" {
		// transcoder.FFprobeBinary = fmt.Sprintf("%s/utils/ffprobe", baseDir)
		transcoder.FFprobeBinary = "ffprobe"
	}

	return &transcoder
}

func (t *Transcoder) Transcode() error {
	t.playlist = playlist(t.VideoProfiles, "%s/%s.m3u8")
	err := os.WriteFile(path.Join(t.TranscodeDir, "playlist.m3u8"), []byte(t.playlist), 0644)
	if err != nil {
		log.Error().Err(err).Msg("Error writing playlist")
	}

	err = t.fetchMetadata()
	if err != nil {
		log.Error().Err(err).Msg("Error fetching metadata")
	}

	for name, profile := range t.VideoProfiles {
		start := time.Now()
		if _, err := os.Stat(path.Join(t.TranscodeDir, name)); os.IsNotExist(err) {
			err = os.Mkdir(path.Join(t.TranscodeDir, name), 0755)
			if err != nil {
				log.Error().Err(err).Str("name", name).Msg("Error creating transcode directory")
			}
		}

		args := []string{
			"-loglevel", "warning",
		}

		// Input specs
		args = append(args, []string{
			"-i", t.MediaUrl, // Input file
			"-force_key_frames", "expr:gte(t,n_forced*10)",
		}...)

		// Video specs
		var scale string
		if profile.Width >= profile.Height {
			scale = fmt.Sprintf("scale=-2:%d", profile.Height)
		} else {
			scale = fmt.Sprintf("scale=%d:-2", profile.Width)
		}
		threads := runtime.NumCPU()
		if threads <= 0 {
			threads = 1
		} else {
			threads = threads - 1
			if threads > 16 {
				threads = 16
			}
		}

		args = append(args, []string{
			"-vf", scale,
			// "-preset", "veryslow",
			"-preset", "fast",
			// "-profile:v", "high",
			// "-level:v", "4.0",
			"-r", "25",
			"-b:v", fmt.Sprintf("%dk", profile.Bitrate),
			"-threads", fmt.Sprintf("%d", threads),
			"-crf", fmt.Sprintf("%d", profile.Quality),
			"-minrate", fmt.Sprintf("%fk", float64(profile.Bitrate)*0.8),
			"-maxrate", fmt.Sprintf("%fk", float64(profile.Bitrate)*1.2),
			"-bufsize", fmt.Sprintf("%dk", profile.Bitrate*2),
		}...)

		if strings.HasPrefix(name, "vp9") {
			args = append(args, []string{
				"-c:v", "libvpx-vp9",
				"-hls_segment_type", "fmp4",
				"-tag:v", "vp09",
				"-tile-columns", "2",
				// "-tile-rows", "2",
			}...)
		} else {
			args = append(args, []string{
				"-c:v", "libx264",
				"-hls_segment_type", "mpegts",
				"-tag:v", "avc1.42E01E",
				"-tune", "zerolatency",
			}...)
		}

		// Audio specs
		args = append(args, []string{
			"-c:a", "aac",
			"-b:a", fmt.Sprintf("%dk", t.AudioProfile.Bitrate),
		}...)

		// Hls specs
		args = append(args, []string{
			"-f", "hls",
			"-movflags", "+faststart",
			"-hls_time", fmt.Sprintf("%.2f", t.segmentLength),
			"-hls_list_size", "0",
			"-hls_flags", "split_by_time",
			"-hls_playlist_type", "vod",
			"-hls_segment_filename",
			path.Join(t.TranscodeDir, name, fmt.Sprintf("%s-%%05d.ts", t.segmentPrefix)),
			path.Join(t.TranscodeDir, name, fmt.Sprintf("%s.m3u8", name)),
		}...)

		cmd := exec.CommandContext(t.ctx, t.FFmpegBinary, args...)
		// log.Info().Msgf("Starting FFmpeg process with args %s - profile %s", strings.Join(cmd.Args[:], " "), name)
		cmd.Stdout = slog.Writer()
		cmd.Stderr = slog.Writer()
		// Start the command
		if err := cmd.Start(); err != nil {
			log.Error().Err(err).Str("name", name).Msg("Error starting ffmpeg command")
		}

		// Wait for the command to finish
		if err := cmd.Wait(); err != nil {
			log.Error().Err(err).Str("name", name).Msg("Error waiting for ffmpeg command")
		}
		log.Info().Str("name", name).Interface("duration", time.Since(start).Seconds()).Msgf("Finished FFmpeg")
	}
	return nil
}

// fetch metadata using ffprobe
func (t *Transcoder) fetchMetadata() (err error) {
	start := time.Now()
	log.Info().Msg("fetching metadata")

	// start ffprobe to get metadata about current media
	t.metadata, err = ProbeMedia(t.ctx, t.FFprobeBinary, t.MediaUrl)
	if err != nil {
		return fmt.Errorf("unable probe media for metadata: %v", err)
	}

	// if media has video, use keyframes as reference for segments if allowed so
	if t.metadata.Video != nil && t.metadata.Video.PktPtsTime == nil && t.VideoKeyframes {
		// start ffprobe to get keyframes from video
		videoData, err := ProbeVideo(t.ctx, t.FFprobeBinary, t.MediaUrl)
		if err != nil {
			return fmt.Errorf("unable probe video for keyframes: %v", err)
		}
		t.metadata.Video.PktPtsTime = videoData.PktPtsTime
	}

	elapsed := time.Since(start).Seconds()
	log.Info().Interface("duration", elapsed).Msg("fetched metadata")
	return
}

func playlist(profiles map[string]VideoProfile, segmentNameFmt string) string {
	layers := []struct {
		Bitrate int
		Entries []string
	}{}

	for name, profile := range profiles {
		codecs := ""
		if strings.HasPrefix(name, "vp9") {
			codecs = "vp09.00.10.08"
		} else {
			codecs = "avc1.42E01E"
		}
		layers = append(layers, struct {
			Bitrate int
			Entries []string
		}{
			profile.Bitrate,
			[]string{
				fmt.Sprintf(`#EXT-X-STREAM-INF:BANDWIDTH=%d,CODECS="%s,mp4a.40.2",RESOLUTION=%dx%d,NAME=%s`, profile.Bitrate, codecs, profile.Width, profile.Height, name),
				fmt.Sprintf(segmentNameFmt, name, name),
			},
		})
	}
	//CODECS="{codec_tags[codec]},{audio_codec_tag}"

	// sort by bitrate
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].Bitrate < layers[j].Bitrate
	})

	// playlist prefix
	playlist := []string{"#EXTM3U"}

	// playlist segments
	for _, profile := range layers {
		playlist = append(playlist, profile.Entries...)
	}

	// join with newlines
	return strings.Join(playlist, "\n")
}

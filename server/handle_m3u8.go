package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

func (s *Server) MasterPlaylist(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	// Extract video_id from URL parameters.
	videoID := chi.URLParam(r, "video_id")
	log.Info().Str("video_id", videoID).Msg("master playlist requested")

	// Construct the path to the master.m3u8 file.
	filePath := fmt.Sprintf("./output/%s/playlist.m3u8", videoID)

	return filePath, nil
}

func (s *Server) Master(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	// Extract video_id from URL parameters.
	videoID := chi.URLParam(r, "video_id")
	fileName := chi.URLParam(r, "file_name")
	bitrate := chi.URLParam(r, "bitrate")
	log.Info().Str("video_id", videoID).Msg("master playlist requested")
	log.Info().Str("file_path", fileName).Msg("master playlist requested")
	log.Info().Str("bitrate", bitrate).Msg("master playlist requested")

	// Construct the path to the master.m3u8 file.
	filePath := fmt.Sprintf("./output/%s/%s/%s", videoID, bitrate, fileName)

	return filePath, nil
}

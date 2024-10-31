package server

import "github.com/go-chi/chi/v5"

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()

	// Service health checks.
	r.Get("/", Health)
	r.Get("/healthz", Health)

	r.Get("/video/{video_id}/master.m3u8", M8u8(s.MasterPlaylist))
	r.Get("/video/{video_id}/{bitrate}/{file_name}", M8u8(s.Master))
	return r
}

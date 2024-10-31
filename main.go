package main

import (
	"context"
	"fmt"
	stdlog "log"
	"os"

	"github.com/joeshaw/envdecode"
	"github.com/nayan9229/hls-transcoder/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var appname = "transcoding-service"

var release string

func main() {

	var cfg server.Config

	if err := envdecode.StrictDecode(&cfg); err != nil {
		log.Fatal().Err(err).
			Msg("failed to process environment variables")
	}

	cfg.AppName = appname
	cfg.Release = release

	LogSetup(appname, cfg.Environment == "dev")

	cmd := newCommand(&cfg)

	if err := cmd.Execute(); err != nil {
		die(err)
	}
}

func die(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func newCommand(cfg *server.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "message-service",
		Short: "Message brokering and channel management",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			var (
				srv = server.NewServer(cfg)
				ctx = context.Background()
			)

			go srv.ProcessTranscoding(ctx)

			srv.Serve()
		},
	}
	return cmd
}

func LogSetup(appname string, dev bool) {
	baselog := zerolog.New(os.Stdout)
	if dev {
		baselog = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout})
	}
	applog := baselog.With().Timestamp().Str("service", appname).Logger()
	log.Logger = applog

	stdlog.SetFlags(0)
	stdlog.SetOutput(applog.With().Logger())
}

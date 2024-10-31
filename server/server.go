package server

import (
	"context"
	"encoding/json"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	sentry "github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

type Server struct {
	Port     string
	AppName  string
	Ctx      context.Context
	Srv      *http.Server
	muAtExit sync.Mutex
	atExit   []func()
	config   Config
	wtick    chan struct{} // ticker for polling for work
}

type Config struct {
	AppName        string
	Project        string `env:"PROJECT_ID,default=dev"`
	Port           int    `env:"PORT,default=8080"`
	Credentials    string `env:"CREDENTIALS_PATH"`
	SentryDSN      string `env:"SENTRY_DSN"`
	SentryTracing  bool   `env:"SENTRY_TRACING,default=false"`
	Environment    string `env:"ENVIRONMENT,default=dev"`
	MaxConnections int    `env:"MAX_CONNECTIONS,default=50"`
	Release        string
}

// Init initialises all the common infrastructure used by REST
// servers.
func (s *Server) Init(cfg *Config, r chi.Router) {
	// Randomise ID generation.
	rand.Seed(int64(time.Now().Nanosecond()))

	s.AppName = cfg.AppName
	s.Ctx = context.Background()
	s.atExit = []func(){}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.SentryDSN,
		Debug:            false,
		AttachStacktrace: true,
		Release:          cfg.Release,
		EnableTracing:    cfg.SentryTracing,
		TracesSampleRate: 1.0,
		TracesSampler: func(ctx sentry.SamplingContext) float64 {
			switch ctx.Span.Name {
			case "GET /healthz", "GET /":
				return 0.0
			default:
				return 1.0
			}
		},
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			event.Tags["environment"] = cfg.Environment
			return event
		},
		BeforeSendTransaction: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			event.Tags["environment"] = cfg.Environment

			switch event.Transaction {
			case "GET /healthz", "GET /":
				return nil
			default:
				return event
			}
		},
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create Sentry client")
	}

	sentry.CurrentHub().Scope().SetTag("app", cfg.AppName)
	sentry.CurrentHub().Scope().SetTag("environment", cfg.Environment)

	s.Srv = &http.Server{
		DisableGeneralOptionsHandler: false,
		Handler:                      sentryhttp.New(sentryhttp.Options{}).Handle(r),
		Addr:                         net.JoinHostPort("", strconv.Itoa(cfg.Port)),
		ReadTimeout:                  30 * time.Second,
		ReadHeaderTimeout:            10 * time.Second,
		WriteTimeout:                 30 * time.Second,
	}

}

// Healthcheck endpoints for simple servers.
func (s *Server) HealthRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", Health)
	r.Get("/healthz", Health)
	return r
}

// Serve runs a server event loop.
func (s *Server) Serve() {
	errChan := make(chan error, 0)
	go func() {
		log.Info().
			Str("address", s.Srv.Addr).
			Msg("server started")
		err := s.Srv.ListenAndServe()
		if err != nil {
			errChan <- err
		}
	}()

	signalCh := make(chan os.Signal, 0)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)

	var err error

	select {
	case <-signalCh:
	case err = <-errChan:
	}

	s.shutdown()
	s.runAtShutdown()

	if err == nil {
		log.Info().Msg("server shutting down")
	} else {
		log.Fatal().Err(err).Msg("server failed")
	}
}

// AddAtExit adds an exit handler function.
func (s *Server) AddAtExit(fn func()) {
	s.muAtExit.Lock()
	defer s.muAtExit.Unlock()
	s.atExit = append(s.atExit, fn)
}

// Shut down server.
func (s *Server) shutdown() {
	ctx, cancel := context.WithTimeout(s.Ctx, 10*time.Second)
	defer cancel()
	if err := s.Srv.Shutdown(ctx); err != nil {
		log.Error().Err(err)
	}
}

// Run at-exit processing.
func (s *Server) runAtShutdown() {
	s.muAtExit.Lock()
	defer s.muAtExit.Unlock()
	for _, fn := range s.atExit {
		fn()
	}
}

// SimpleHandlerFunc is a HTTP handler function that signals internal
// errors by returning a normal Go error, and when successful returns
// a response body to be marshalled to JSON. It can be wrapped in the
// SimpleHandler middleware to produce a normal HTTP handler function.
type SimpleHandlerFunc func(w http.ResponseWriter, r *http.Request) (interface{}, error)

// SimpleHandler wraps a simpleHandler-style HTTP handler function to
// turn it into a normal HTTP handler function. Go errors from the
// inner handler are returned to the caller as "500 Internal Server
// Error" responses. Returns from successful processing by the inner
// handler and marshalled into a JSON response body.
func SimpleHandler(inner SimpleHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Run internal handler: returns a marshalable result and an
		// error, either of which may be nil.
		result, err := inner(w, r)

		// Propagate Go errors as "500 Internal Server Error" responses.
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("handling %q: %v", r.RequestURI, err)
			return
		}

		// No response body, so internal handler dealt with response
		// setup.
		if result == nil {
			return
		}

		// Marshal JSON response body.
		body, err := json.Marshal(result)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("handling %q: %v", r.RequestURI, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}
}

func M8u8(inner func(w http.ResponseWriter, r *http.Request) (interface{}, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Run the inner handler to get the file path and any error.
		result, err := inner(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Printf("handling %q: %v", r.RequestURI, err)
			return
		}

		// Cast result to a string (expected to be the file path).
		filePath, ok := result.(string)
		if !ok {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			log.Printf("handling %q: result is not a valid file path", r.RequestURI)
			return
		}

		// Set headers for serving .m3u8 file.
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Header().Set("Cache-Control", "no-cache")

		// Serve the file from the file path.
		http.ServeFile(w, r, filePath)
	}
}

func NewServer(cfg *Config) *Server {
	srv := &Server{
		config: *cfg,
		wtick:  make(chan struct{}, 1),
	}
	srv.Init(cfg, srv.routes())
	return srv
}

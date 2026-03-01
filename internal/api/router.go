package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/ilyastar9999/heCsTackForse/internal/ad"
	"github.com/ilyastar9999/heCsTackForse/internal/config"
	"github.com/ilyastar9999/heCsTackForse/internal/db"
	"github.com/ilyastar9999/heCsTackForse/internal/deployer"
)

type Server struct {
	cfg         *config.Config
	db          *db.DB
	deployer    *deployer.Manager
	adEngine    *ad.Engine
	r           *chi.Mux
	rateLimiter *rateLimiter
}

func NewServer(cfg *config.Config, database *db.DB, mgr *deployer.Manager, adEng *ad.Engine) *Server {
	s := &Server{cfg: cfg, db: database, deployer: mgr, adEngine: adEng, rateLimiter: newRateLimiter()}
	s.r = s.buildRouter()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.r.ServeHTTP(w, r)
}

func (s *Server) buildRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Serve frontend static files
	fs := http.FileServer(http.Dir(s.cfg.Server.StaticDir))
	r.Handle("/static/*", http.StripPrefix("/static", fs))

	// Serve HTML pages
	r.Get("/", serveFile(s.cfg.Server.StaticDir+"/index.html"))
	r.Get("/login", serveFile(s.cfg.Server.StaticDir+"/login.html"))
	r.Get("/register", serveFile(s.cfg.Server.StaticDir+"/register.html"))
	r.Get("/scoreboard", serveFile(s.cfg.Server.StaticDir+"/scoreboard.html"))
	r.Get("/challenges", serveFile(s.cfg.Server.StaticDir+"/challenges.html"))
	r.Get("/profile", serveFile(s.cfg.Server.StaticDir+"/profile.html"))
	r.Get("/admin", serveFile(s.cfg.Server.StaticDir+"/admin.html"))
	r.Get("/ad", serveFile(s.cfg.Server.StaticDir+"/ad.html"))

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Public
		r.Post("/auth/register", s.handleRegister)
		r.Post("/auth/login", s.handleLogin)
		r.Get("/scoreboard", s.handleScoreboard)

		// Authenticated
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Post("/auth/logout", s.handleLogout)
			r.Get("/auth/me", s.handleMe)

			r.Get("/challenges", s.handleListChallenges)
			r.Get("/challenges/{id}", s.handleGetChallenge)
			r.With(s.rateLimitMiddleware(10, time.Minute)).Post("/challenges/{id}/submit", s.handleSubmitFlag)

			r.Get("/users/{id}", s.handleGetUser)
			r.Post("/teams", s.handleCreateTeam)
			r.Post("/teams/join", s.handleJoinTeam)
			r.Get("/teams/{id}", s.handleGetTeam)

			// Attack & Defence
			r.Get("/ad/status", s.handleADStatus)
			r.Get("/ad/scoreboard", s.handleADScoreboard)
			r.Get("/ad/services", s.handleADServices)
			r.Get("/ad/vpn", s.handleADGetVPN)
			r.Get("/ad/sploits", s.handleADListSploits)
			r.Post("/ad/sploits", s.handleADCreateSploit)
			r.Put("/ad/sploits/{id}", s.handleADUpdateSploit)
			r.Delete("/ad/sploits/{id}", s.handleADDeleteSploit)
			r.Get("/ad/sploits/{id}/results", s.handleADSploitResults)
			r.With(s.rateLimitMiddleware(20, 60*time.Second)).Post("/ad/flags/submit", s.handleADSubmitFlag)
		})

		// Admin
		r.Group(func(r chi.Router) {
			r.Use(s.authMiddleware)
			r.Use(s.adminMiddleware)
			r.Get("/admin/challenges", s.handleAdminListChallenges)
			r.Post("/admin/challenges", s.handleAdminCreateChallenge)
			r.Put("/admin/challenges/{id}", s.handleAdminUpdateChallenge)
			r.Delete("/admin/challenges/{id}", s.handleAdminDeleteChallenge)
			r.Get("/admin/users", s.handleAdminListUsers)
			r.Put("/admin/users/{id}", s.handleAdminUpdateUser)
		})
	})

	return r
}

func serveFile(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	jsonResponse(w, status, map[string]string{"error": msg})
}

// jsonBody wraps a byte slice as an io.Reader (avoids importing bytes in ad.go).
func jsonBody(b []byte) io.Reader {
	return bytes.NewReader(b)
}

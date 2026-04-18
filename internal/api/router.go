package api

import (
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"github.com/ilyastar9999/heCsTackForse/internal/ad"
	"github.com/ilyastar9999/heCsTackForse/internal/config"
	"github.com/ilyastar9999/heCsTackForse/internal/db"
	"github.com/ilyastar9999/heCsTackForse/internal/deployer"
	"github.com/ilyastar9999/heCsTackForse/internal/plugin"
)

type Server struct {
	cfg         *config.Config
	db          *db.DB
	deployer    *deployer.Manager
	adEngine    *ad.Engine
	e           *echo.Echo
	rateLimiter *rateLimiter
}

func NewServer(cfg *config.Config, database *db.DB, mgr *deployer.Manager, adEng *ad.Engine) *Server {
	s := &Server{cfg: cfg, db: database, deployer: mgr, adEngine: adEng, rateLimiter: newRateLimiter()}
	s.e = s.buildRouter()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.e.ServeHTTP(w, r)
}

func (s *Server) buildRouter() *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	e.Use(echomw.GzipWithConfig(echomw.GzipConfig{Level: 5}))

	// Serve frontend static files
	e.Static("/static", s.cfg.Server.StaticDir+"/static")

	// Serve HTML pages
	sd := s.cfg.Server.StaticDir
	e.GET("/", func(c echo.Context) error { return c.File(sd + "/index.html") })
	e.GET("/login", func(c echo.Context) error { return c.File(sd + "/login.html") })
	e.GET("/register", func(c echo.Context) error { return c.File(sd + "/register.html") })
	e.GET("/scoreboard", func(c echo.Context) error { return c.File(sd + "/scoreboard.html") })
	e.GET("/challenges", func(c echo.Context) error { return c.File(sd + "/challenges.html") })
	e.GET("/profile", func(c echo.Context) error { return c.File(sd + "/profile.html") })
	e.GET("/admin", func(c echo.Context) error { return c.File(sd + "/admin.html") })
	e.GET("/ad", func(c echo.Context) error { return c.File(sd + "/ad.html") })

	// API routes — public
	api := e.Group("/api")
	api.POST("/auth/register", s.handleRegister)
	api.POST("/auth/login", s.handleLogin)
	api.GET("/scoreboard", s.handleScoreboard)
	api.GET("/config", s.handleConfig)
	api.GET("/plugins", s.handlePlugins)
	api.GET("/notifications", s.handleListNotifications)
	api.GET("/pages", s.handleListPages)
	api.GET("/pages/:slug", s.handleGetPage)
	api.GET("/statistics", s.handleStatistics)
	api.GET("/theme", s.handleGetTheme)
	api.GET("/challenge-types", s.handleChallengeTypes)

	// Authenticated
	auth := api.Group("", s.authMiddleware)
	auth.POST("/auth/logout", s.handleLogout)
	auth.GET("/auth/me", s.handleMe)
	auth.PUT("/auth/me", s.handleUpdateMe)
	auth.PUT("/auth/me/password", s.handleChangePassword)

	auth.GET("/challenges", s.handleListChallenges)
	auth.GET("/challenges/:id", s.handleGetChallenge)
	auth.POST("/challenges/:id/submit", s.handleSubmitFlag, s.rateLimitMiddleware(10, time.Minute))
	auth.GET("/challenges/:id/instance", s.handleGetInstance)
	auth.POST("/challenges/:id/instance", s.handleStartInstance)
	auth.DELETE("/challenges/:id/instance", s.handleStopInstance)
	auth.GET("/challenges/:id/hints", s.handleListChallengeHints)
	auth.GET("/challenges/:id/files", s.handleListChallengeFiles)

	auth.GET("/users/:id", s.handleGetUser)
	auth.POST("/teams", s.handleCreateTeam)
	auth.POST("/teams/join", s.handleJoinTeam)
	auth.GET("/teams/:id", s.handleGetTeam)

	// Attack & Defence
	auth.GET("/ad/status", s.handleADStatus)
	auth.GET("/ad/scoreboard", s.handleADScoreboard)
	auth.GET("/ad/services", s.handleADServices)
	auth.GET("/ad/vpn", s.handleADGetVPN)
	auth.GET("/ad/sploits", s.handleADListSploits)
	auth.POST("/ad/sploits", s.handleADCreateSploit)
	auth.PUT("/ad/sploits/:id", s.handleADUpdateSploit)
	auth.DELETE("/ad/sploits/:id", s.handleADDeleteSploit)
	auth.GET("/ad/sploits/:id/results", s.handleADSploitResults)
	auth.POST("/ad/flags/submit", s.handleADSubmitFlag, s.rateLimitMiddleware(20, 60*time.Second))

	// Admin
	admin := api.Group("/admin", s.authMiddleware, s.adminMiddleware)
	admin.GET("/challenges", s.handleAdminListChallenges)
	admin.POST("/challenges", s.handleAdminCreateChallenge)
	admin.PUT("/challenges/:id", s.handleAdminUpdateChallenge)
	admin.DELETE("/challenges/:id", s.handleAdminDeleteChallenge)
	admin.GET("/challenges/:id/flags", s.handleAdminListChallengeFlags)
	admin.POST("/challenges/:id/flags", s.handleAdminCreateChallengeFlag)
	admin.DELETE("/challenges/:id/flags/:fid", s.handleAdminDeleteChallengeFlag)
	admin.GET("/challenges/:id/hints", s.handleAdminListChallengeHints)
	admin.POST("/challenges/:id/hints", s.handleAdminCreateChallengeHint)
	admin.DELETE("/challenges/:id/hints/:hid", s.handleAdminDeleteChallengeHint)
	admin.GET("/challenges/:id/files", s.handleAdminListChallengeFiles)
	admin.POST("/challenges/:id/files", s.handleAdminUploadFile)
	admin.DELETE("/challenges/:id/files/:fid", s.handleAdminDeleteChallengeFile)
	admin.GET("/users", s.handleAdminListUsers)
	admin.GET("/users/:id", s.handleAdminGetUser)
	admin.PUT("/users/:id", s.handleAdminUpdateUser)
	admin.DELETE("/users/:id", s.handleAdminDeleteUser)
	admin.POST("/users/:id/reset_score", s.handleAdminResetScore)
	admin.PUT("/users/:id/fields/:fid", s.handleAdminSetUserFieldValue)
	admin.GET("/notifications", s.handleAdminListNotifications)
	admin.POST("/notifications", s.handleAdminCreateNotification)
	admin.DELETE("/notifications/:id", s.handleAdminDeleteNotification)
	admin.GET("/pages", s.handleAdminListPages)
	admin.POST("/pages", s.handleAdminCreatePage)
	admin.PUT("/pages/:id", s.handleAdminUpdatePage)
	admin.DELETE("/pages/:id", s.handleAdminDeletePage)
	admin.GET("/config", s.handleAdminGetConfig)
	admin.PUT("/config", s.handleAdminSetConfig)
	admin.PUT("/theme", s.handleAdminSetTheme)
	admin.GET("/userfields", s.handleAdminListUserFields)
	admin.POST("/userfields", s.handleAdminCreateUserField)
	admin.DELETE("/userfields/:id", s.handleAdminDeleteUserField)

	return e
}

// jsonBody wraps a byte slice as an io.Reader (avoids importing bytes in ad.go).
func jsonBody(b []byte) io.Reader {
	return bytes.NewReader(b)
}

// handleConfig returns public CTF configuration so the frontend can adapt.
func (s *Server) handleConfig(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"name":              s.cfg.CTF.Name,
		"mode":              s.cfg.CTF.Mode,
		"team_mode":         s.cfg.CTF.TeamMode,
		"scoring":           s.cfg.CTF.Scoring,
		"flag_prefix":       s.cfg.CTF.FlagPrefix,
		"flag_suffix":       s.cfg.CTF.FlagSuffix,
		"registration_open": s.cfg.CTF.RegistrationOpen,
		"language":          s.cfg.CTF.Language,
	})
}

// handlePlugins returns the names of all plugins registered in the Default registry.
func (s *Server) handlePlugins(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"scorers":       plugin.Default.ScorerNames(),
		"flag_checkers": plugin.Default.FlagCheckerNames(),
		"notifiers":     plugin.Default.NotifierNames(),
	})
}

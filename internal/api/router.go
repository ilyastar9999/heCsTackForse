package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"github.com/ilyastar9999/heCsTackForse/internal/ad"
	"github.com/ilyastar9999/heCsTackForse/internal/cache"
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
	cache       *cache.Cache
	e           *echo.Echo
	rateLimiter *rateLimiter
}

func NewServer(cfg *config.Config, database *db.DB, mgr *deployer.Manager, adEng *ad.Engine, c *cache.Cache) *Server {
	s := &Server{cfg: cfg, db: database, deployer: mgr, adEngine: adEng, cache: c, rateLimiter: newRateLimiter()}
	if err := s.bootstrapDeployScheduler(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to bootstrap deploy scheduler: %v\n", err)
	}
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
	e.Use(echomw.SecureWithConfig(echomw.SecureConfig{
		XSSProtection:         "0",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		ContentSecurityPolicy: "default-src 'self'; img-src 'self' data:; connect-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; font-src 'self' data:; base-uri 'self'; frame-ancestors 'none'",
	}))

	// Serve frontend static files
	e.Static("/static", s.cfg.Server.StaticDir+"/static")
	e.Static("/ctfd-themes", "themes")

	// Serve HTML pages
	sd := s.cfg.Server.StaticDir
	e.GET("/", func(c echo.Context) error {
		if s.setupRequired() {
			return c.Redirect(http.StatusFound, "/setup")
		}
		return c.File(sd + "/index.html")
	})
	e.GET("/setup", func(c echo.Context) error {
		if !s.setupRequired() {
			return c.Redirect(http.StatusFound, "/")
		}
		return c.File(sd + "/setup.html")
	})
	e.GET("/login", func(c echo.Context) error { return c.File(sd + "/login.html") })
	e.GET("/register", func(c echo.Context) error { return c.File(sd + "/register.html") })
	e.GET("/scoreboard", func(c echo.Context) error { return c.File(sd + "/scoreboard.html") })
	e.GET("/challenges", func(c echo.Context) error { return c.File(sd + "/challenges.html") })
	e.GET("/modules", func(c echo.Context) error { return c.File(sd + "/modules.html") })
	e.GET("/profile", func(c echo.Context) error { return c.File(sd + "/profile.html") })
	e.GET("/admin", func(c echo.Context) error { return c.File(sd + "/admin.html") })
	e.GET("/admin/deploy", func(c echo.Context) error { return c.File(sd + "/admin-deploy.html") })
	e.GET("/admin/plugins", func(c echo.Context) error { return c.File(sd + "/admin-plugins.html") })
	e.GET("/admin/plugins/:slug", s.handleAdminPluginPage)
	e.GET("/admin/users/:id", func(c echo.Context) error { return c.File(sd + "/admin-user.html") })
	e.GET("/ad", func(c echo.Context) error { return c.Redirect(http.StatusFound, "/challenges") })
	e.GET("/notifications", func(c echo.Context) error { return c.File(sd + "/notifications.html") })
	e.GET("/statistics", func(c echo.Context) error { return c.File(sd + "/statistics.html") })
	e.GET("/page", func(c echo.Context) error { return c.File(sd + "/page.html") })

	// Health checks
	e.GET("/healthz", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	e.GET("/ready", func(c echo.Context) error {
		if err := s.db.Ping(); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"status": "not ready"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ready"})
	})

	// API routes — public
	api := e.Group("/api")
	api.POST("/auth/register", s.handleRegister, s.rateLimitMiddleware(5, time.Minute))
	api.POST("/auth/login", s.handleLogin, s.rateLimitMiddleware(10, time.Minute))
	api.GET("/setup/status", s.handleSetupStatus)
	api.POST("/setup", s.handleSetup)
	api.GET("/scoreboard", s.handleScoreboard)
	api.GET("/config", s.handleConfig)
	api.GET("/plugins", s.handlePlugins)
	api.GET("/notifications", s.handleListNotifications)
	api.GET("/pages", s.handleListPages)
	api.GET("/pages/:slug", s.handleGetPage)
	api.GET("/modules", s.handleListModules)
	api.GET("/modules/:slug", s.handleGetModule)
	api.GET("/statistics", s.handleStatistics)
	api.GET("/theme", s.handleGetTheme)
	api.GET("/challenge-types", s.handleChallengeTypes)

	// Authenticated
	auth := api.Group("", s.authMiddleware, s.csrfOriginMiddleware)
	auth.POST("/auth/logout", s.handleLogout)
	auth.GET("/auth/me", s.handleMe)
	auth.PUT("/auth/me", s.handleUpdateMe)
	auth.PUT("/auth/me/password", s.handleChangePassword)

	auth.GET("/challenges", s.handleListChallenges)
	auth.GET("/challenges/:id", s.handleGetChallenge)
	auth.POST("/challenges/:id/submit", s.handleSubmitFlag, s.rateLimitMiddleware(10, time.Minute))
	auth.POST("/flags/submit", s.handleSubmitAnyFlag, s.rateLimitMiddleware(10, time.Minute))
	auth.GET("/challenges/:id/instance", s.handleGetInstance)
	auth.POST("/challenges/:id/instance", s.handleStartInstance)
	auth.POST("/challenges/:id/instance/extend", s.handleExtendInstance)
	auth.DELETE("/challenges/:id/instance", s.handleStopInstance)
	auth.GET("/challenges/:id/hints", s.handleListChallengeHints)
	auth.GET("/challenges/:id/files", s.handleListChallengeFiles)
	auth.GET("/challenges/:id/workflow", s.handleChallengeWorkflow)
	auth.POST("/challenges/:id/workflow/check", s.handleChallengeWorkflowCheck, s.rateLimitMiddleware(10, time.Minute))
	auth.POST("/challenges/:id/workflow/restart-vote", s.handleChallengeWorkflowRestartVote, s.rateLimitMiddleware(10, time.Minute))
	auth.POST("/challenges/:id/workflow/restart", s.handleChallengeWorkflowAdminRestart, s.rateLimitMiddleware(10, time.Minute))

	// Per-challenge AD routes
	auth.GET("/challenges/:id/sploits", s.handleADListChallengeSploits)
	auth.POST("/challenges/:id/sploits", s.handleADCreateChallengeSploit)
	auth.GET("/challenges/:id/vpn", s.handleADGetVPN)
	auth.GET("/challenges/:id/vpn/status", s.handleADVPNStatus)
	auth.POST("/challenges/:id/vpn/sync", s.handleADSyncVPN, s.rateLimitMiddleware(10, time.Minute))

	auth.GET("/users/:id", s.handleGetUser)
	auth.POST("/teams", s.handleCreateTeam)
	auth.POST("/teams/join", s.handleJoinTeam)
	auth.GET("/teams/:id", s.handleGetTeam)

	// Attack & Defence
	auth.GET("/ad/status", s.handleADStatus)
	auth.GET("/ad/scoreboard", s.handleADScoreboard)
	auth.GET("/ad/catalog", s.handleADCatalog)
	auth.GET("/ad/services", s.handleADServices)
	auth.GET("/ad/vpn/status", s.handleADVPNStatus)
	auth.GET("/ad/vpn", s.handleADGetVPN)
	auth.POST("/ad/vpn/sync", s.handleADSyncVPN, s.rateLimitMiddleware(10, time.Minute))
	auth.GET("/ad/sploits", s.handleADListSploits)
	auth.POST("/ad/sploits", s.handleADCreateSploit)
	auth.PUT("/ad/sploits/:id", s.handleADUpdateSploit)
	auth.DELETE("/ad/sploits/:id", s.handleADDeleteSploit)
	auth.GET("/ad/sploits/:id/results", s.handleADSploitResults)
	auth.POST("/ad/flags/submit", s.handleADSubmitFlag, s.rateLimitMiddleware(20, 60*time.Second))

	// Admin
	admin := api.Group("/admin", s.authMiddleware, s.adminMiddleware, s.csrfOriginMiddleware)
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
	admin.GET("/modules", s.handleAdminListModules)
	admin.POST("/modules", s.handleAdminCreateModule)
	admin.PUT("/modules/:id", s.handleAdminUpdateModule)
	admin.DELETE("/modules/:id", s.handleAdminDeleteModule)
	admin.POST("/modules/:id/items", s.handleAdminCreateModuleItem)
	admin.PUT("/modules/items/:item_id", s.handleAdminUpdateModuleItem)
	admin.DELETE("/modules/items/:item_id", s.handleAdminDeleteModuleItem)
	admin.GET("/config", s.handleAdminGetConfig)
	admin.PUT("/config", s.handleAdminSetConfig)
	admin.GET("/deploy", s.handleAdminGetDeployConfig)
	admin.PUT("/deploy", s.handleAdminSetDeployConfig)
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
	ctfName := s.cfg.CTF.Name
	ctfMode := s.cfg.CTF.Mode
	language := s.cfg.CTF.Language
	registrationOpen := s.cfg.CTF.RegistrationOpen
	teamMode := s.cfg.CTF.TeamMode
	if settings := s.publicSettings(); len(settings) > 0 {
		if value, ok := settings["ctf_name"].(string); ok && strings.TrimSpace(value) != "" {
			ctfName = value
		}
		if value, ok := settings["ctf_mode"].(string); ok && (value == "ctf" || value == "ad") {
			ctfMode = value
		}
		if value, ok := settings["language"].(string); ok && strings.TrimSpace(value) != "" {
			language = value
		}
		if value, ok := settings["registration_open"].(bool); ok {
			registrationOpen = value
		}
		if value, ok := settings["team_mode"].(bool); ok {
			teamMode = value
		}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"name":              ctfName,
		"mode":              ctfMode,
		"team_mode":         teamMode,
		"scoring":           s.cfg.CTF.Scoring,
		"flag_prefix":       s.cfg.CTF.FlagPrefix,
		"flag_suffix":       s.cfg.CTF.FlagSuffix,
		"registration_open": registrationOpen,
		"language":          language,
		"instance_ttl":      s.cfg.Deployer.InstanceTTL,
	})
}

func (s *Server) publicSettings() map[string]any {
	rows, err := s.db.Query(`SELECT key, value FROM ctf_settings WHERE key IN ('ctf_name', 'ctf_mode', 'language', 'registration_open', 'team_mode')`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	settings := map[string]any{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err == nil {
			settings[key] = normalizeAdminConfigValue(key, value)
		}
	}
	return settings
}

// handlePlugins returns the names of all plugins registered in the Default registry.
func (s *Server) handlePlugins(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{
		"scorers":              plugin.Default.ScorerNames(),
		"flag_checkers":        plugin.Default.FlagCheckerNames(),
		"notifiers":            plugin.Default.NotifierNames(),
		"challenge_types":      plugin.Default.ChallengeTypeNames(),
		"challenge_type_specs": plugin.Default.ChallengeTypes(),
		"home_widgets":         plugin.Default.HomeWidgets(),
		"admin_menu":           plugin.Default.AdminMenu(),
	})
}

func (s *Server) handleAdminPluginPage(c echo.Context) error {
	slug := strings.ToLower(strings.TrimSpace(c.Param("slug")))
	if slug == "" || !s.isKnownPluginAdminSlug(slug) {
		return echo.NewHTTPError(http.StatusNotFound, "plugin admin page not found")
	}

	filename := "admin-plugin.html"
	if custom, ok := pluginAdminTemplate(slug); ok {
		path := filepath.Join(s.cfg.Server.StaticDir, custom)
		if _, err := os.Stat(path); err == nil {
			filename = custom
		}
	}
	return c.File(filepath.Join(s.cfg.Server.StaticDir, filename))
}

func (s *Server) isKnownPluginAdminSlug(slug string) bool {
	want := "/admin/plugins/" + slug
	for _, item := range plugin.Default.AdminMenu() {
		if strings.EqualFold(strings.TrimSpace(item.Route), want) {
			return true
		}
	}
	return false
}

func pluginAdminTemplate(slug string) (string, bool) {
	if slug == "" {
		return "", false
	}
	for _, r := range slug {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return "", false
		}
	}
	return "admin-plugin-" + strings.ReplaceAll(slug, "_", "-") + ".html", true
}

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ilyastar9999/heCsTackForse/internal/ad"
	"github.com/ilyastar9999/heCsTackForse/internal/api"
	"github.com/ilyastar9999/heCsTackForse/internal/cache"
	"github.com/ilyastar9999/heCsTackForse/internal/config"
	"github.com/ilyastar9999/heCsTackForse/internal/db"
	"github.com/ilyastar9999/heCsTackForse/internal/deployer"
	"github.com/ilyastar9999/heCsTackForse/internal/plugin"
	"github.com/ilyastar9999/heCsTackForse/internal/pluginbridge"
)

func main() {
	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	database, err := db.New(cfg.Database.Driver, cfg.Database.DSN)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	if cfg.CTF.SeedChallenges {
		if err := database.Seed(); err != nil {
			log.Printf("warning: challenge seeding failed: %v", err)
		}
	}

	var bridge *pluginbridge.Bridge
	if cfg.Plugins.Enabled {
		var err error
		bridge, err = pluginbridge.New(pluginbridge.Config{
			Python:     cfg.Plugins.Python,
			Script:     cfg.Plugins.Script,
			PluginDirs: cfg.Plugins.PluginDirs,
			Modules:    cfg.Plugins.Modules,
		})
		if err != nil {
			log.Fatalf("failed to start plugin bridge: %v", err)
		}
		catalog := bridge.Catalog()
		for _, name := range catalog.FlagCheckers {
			plugin.Default.RegisterFlagChecker(bridge.FlagChecker(name))
		}
		for _, name := range catalog.Scorers {
			plugin.Default.RegisterScorer(bridge.Scorer(name))
		}
		for _, name := range catalog.Notifiers {
			plugin.Default.RegisterNotifier(bridge.Notifier(name))
		}
		for _, name := range catalog.ChallengeTypes {
			plugin.Default.RegisterChallengeType(bridge.ChallengeType(name))
		}
		for _, rawWidget := range catalog.HomeWidgets {
			widgetMap, ok := rawWidget.(map[string]any)
			if !ok {
				continue
			}
			name, _ := widgetMap["name"].(string)
			title, _ := widgetMap["title"].(string)
			body, _ := widgetMap["body"].(string)
			url, _ := widgetMap["url"].(string)
			plugin.Default.RegisterHomeWidget(plugin.HomeWidget{
				Name:  name,
				Title: title,
				Body:  body,
				URL:   url,
			})
		}
		for _, rawMenu := range catalog.AdminMenu {
			menuMap, ok := rawMenu.(map[string]any)
			if !ok {
				continue
			}
			name, _ := menuMap["name"].(string)
			title, _ := menuMap["title"].(string)
			route, _ := menuMap["route"].(string)
			plugin.Default.RegisterAdminMenu(plugin.AdminMenuEntry{
				Name:  name,
				Title: title,
				Route: route,
			})
		}
		defer func() {
			if err := bridge.Close(); err != nil {
				log.Printf("warning: plugin bridge shutdown failed: %v", err)
			}
		}()
		log.Printf("loaded plugin bridge: %d flag checkers, %d scorers, %d notifiers, %d challenge types",
			len(catalog.FlagCheckers), len(catalog.Scorers), len(catalog.Notifiers), len(catalog.ChallengeTypes))
	}

	mgr := deployer.NewManager()
	mgr.Register(&deployer.NoDeployDeployer{})

	if cfg.Deployer.Backends.Docker.Enabled {
		mgr.Register(&deployer.DockerDeployer{
			Registry:       cfg.Deployer.Backends.Docker.Registry,
			Host:           cfg.Deployer.Backends.Docker.Host,
			TLSVerify:      cfg.Deployer.Backends.Docker.TLSVerify,
			CertPath:       cfg.Deployer.Backends.Docker.CertPath,
			Network:        cfg.Deployer.Backends.Docker.Network,
			ChallengesDir:  cfg.Deployer.Backends.Docker.ChallengesDir,
			LocalTagPrefix: cfg.Deployer.Backends.Docker.LocalTagPrefix,
		})
	}

	if cfg.Deployer.Backends.Kubernetes.Enabled {
		mgr.Register(&deployer.K8sDeployer{
			Kubeconfig: cfg.Deployer.Backends.Kubernetes.Kubeconfig,
			Namespace:  cfg.Deployer.Backends.Kubernetes.Namespace,
		})
	}

	if cfg.Deployer.Backends.PVE.Enabled {
		mgr.Register(deployer.NewPVEDeployer(
			cfg.Deployer.Backends.PVE.URL,
			cfg.Deployer.Backends.PVE.TokenID,
			cfg.Deployer.Backends.PVE.TokenSecret,
			cfg.Deployer.Backends.PVE.Node,
			cfg.Deployer.Backends.PVE.VMTemplate,
			cfg.Deployer.Backends.PVE.InsecureSkipVerify,
		))
	}

	// Start AD engine (always available; only activates for attack_defence_* challenges)
	var adEngine *ad.Engine
	engine, err := ad.New(database, cfg)
	if err != nil {
		log.Fatalf("failed to create AD engine: %v", err)
	}
	adEngine = engine
	adEngine.Start()

	c := cache.New(cfg.Cache.URL, cfg.Cache.Prefix)

	srv := api.NewServer(cfg, database, mgr, adEngine, c)
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	defer cleanupCancel()
	srv.StartInstanceCleanup(cleanupCtx, time.Minute)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srv,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("starting %s on %s", cfg.CTF.Name, addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	cleanupCancel()
	if adEngine != nil {
		adEngine.Stop()
	}
	c.Close()
	if bridge != nil {
		if err := bridge.Close(); err != nil {
			log.Printf("warning: plugin bridge shutdown failed: %v", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("server stopped")
}

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

	"github.com/ilyastar9999/heCsTackForse/internal/api"
	"github.com/ilyastar9999/heCsTackForse/internal/config"
	"github.com/ilyastar9999/heCsTackForse/internal/db"
	"github.com/ilyastar9999/heCsTackForse/internal/deployer"
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

	database, err := db.New(cfg.Database.DSN)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	if cfg.CTF.SeedChallenges {
		if err := database.Seed(); err != nil {
			log.Printf("warning: challenge seeding failed: %v", err)
		}
	}

	mgr := deployer.NewManager()
	mgr.Register(&deployer.NoopDeployer{})
	mgr.Register(&deployer.NoDeployDeployer{})

	if cfg.Deployer.Backends.Docker.Enabled {
		mgr.Register(&deployer.DockerDeployer{
			Registry: cfg.Deployer.Backends.Docker.Registry,
			Network:  cfg.Deployer.Backends.Docker.Network,
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

	srv := api.NewServer(cfg, database, mgr)

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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
	log.Println("server stopped")
}

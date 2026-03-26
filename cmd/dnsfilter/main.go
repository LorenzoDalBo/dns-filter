package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LorenzoDalBo/dns-filter/internal/api"
	"github.com/LorenzoDalBo/dns-filter/internal/cache"
	"github.com/LorenzoDalBo/dns-filter/internal/captive"
	"github.com/LorenzoDalBo/dns-filter/internal/config"
	dnsserver "github.com/LorenzoDalBo/dns-filter/internal/dns"
	"github.com/LorenzoDalBo/dns-filter/internal/filter"
	"github.com/LorenzoDalBo/dns-filter/internal/identity"
	"github.com/LorenzoDalBo/dns-filter/internal/logger"
	"github.com/LorenzoDalBo/dns-filter/internal/logging"
	"github.com/LorenzoDalBo/dns-filter/internal/store"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "configs/dnsfilter.yaml", "path to config file")
	flag.Parse()

	// Load configuration (RNF04.1)
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Printf("Erro ao carregar configuração: %v\n", err)
		os.Exit(1)
	}

	// Validate JWT secret
	logger.Init(cfg.Log.Level)
	if cfg.API.JWTSecret == "" || cfg.API.JWTSecret == "TROQUE-ESTE-SECRET-EM-PRODUCAO-32chars" {
		fmt.Println("AVISO: JWT secret não configurado! Troque em configs/dnsfilter.yaml ou defina JWT_SECRET")
	}

	// Cache
	dnsCache := cache.New(
		time.Duration(cfg.Cache.TTLFloorSeconds)*time.Second,
		time.Duration(cfg.Cache.TTLCeilingSeconds)*time.Second,
	)

	// Filter
	blacklist := filter.NewBlacklist()
	whitelist := filter.NewBlacklist()

	// PostgreSQL connection
	ctx := context.Background()

	// Auto-migrate database on startup (RNF07.4)
	if err := store.AutoMigrate(cfg.DB.URL); err != nil {
		fmt.Printf("Aviso: auto-migrate falhou: %v\n", err)
	}

	db, err := store.New(ctx, cfg.DB.URL)
	if err != nil {
		fmt.Printf("Aviso: PostgreSQL indisponível (%v) — usando arquivos locais\n", err)
	} else {
		defer db.Close()
		fmt.Println("PostgreSQL: conectado")

		blackDomains, whiteDomains, err := db.LoadActiveBlocklistEntries(ctx)
		if err != nil {
			fmt.Printf("Aviso: erro ao carregar listas do banco: %v\n", err)
		} else {
			for _, d := range blackDomains {
				blacklist.Add(d)
			}
			for _, d := range whiteDomains {
				whitelist.Add(d)
			}
			fmt.Printf("PostgreSQL: %d blacklist + %d whitelist domínios carregados\n",
				len(blackDomains), len(whiteDomains))
		}
	}

	// File fallback (RF04.4)
	if _, err := os.Stat("blocklist.txt"); err == nil {
		count, err := blacklist.LoadFromFile("blocklist.txt")
		if err != nil {
			fmt.Printf("Aviso: erro ao carregar blocklist.txt: %v\n", err)
		} else {
			fmt.Printf("Arquivo: %d domínios carregados de blocklist.txt\n", count)
		}
	}
	if _, err := os.Stat("allowlist.txt"); err == nil {
		count, err := whitelist.LoadFromFile("allowlist.txt")
		if err != nil {
			fmt.Printf("Aviso: erro ao carregar allowlist.txt: %v\n", err)
		} else {
			fmt.Printf("Arquivo: %d domínios carregados de allowlist.txt\n", count)
		}
	}

	filterEngine := filter.NewEngine(blacklist, whitelist)

	// Identity
	identityResolver := identity.NewResolver(1)
	identityResolver.StartSessionEvictor()

	// Log Pipeline
	var logPipeline *logging.Pipeline
	if db != nil {
		logPipeline = logging.NewPipeline(db.Pool(), cfg.DB.LogBufferSize)
		logPipeline.Start()

		// Log retention (RF07.5)
		retention := logging.NewRetention(db.Pool(), time.Duration(cfg.DB.RetentionDays)*24*time.Hour)
		retention.Start()

		// LISTEN/NOTIFY (RF03.9, RNF04.3)
		listener := store.NewListener(db.Pool(), "config_changed")
		listener.Start(ctx, func(payload string) {
			fmt.Printf("Config changed (%s), reloading...\n", payload)
			blackDomains, whiteDomains, err := db.LoadActiveBlocklistEntries(ctx)
			if err != nil {
				fmt.Printf("Reload failed: %v\n", err)
				return
			}
			blacklist.Clear()
			whitelist.Clear()
			for _, d := range blackDomains {
				blacklist.Add(d)
			}
			for _, d := range whiteDomains {
				whitelist.Add(d)
			}
			fmt.Printf("Reloaded: %d blacklist + %d whitelist\n",
				len(blackDomains), len(whiteDomains))
		})
	} else {
		fmt.Println("Log pipeline: desativado (sem PostgreSQL)")
	}

	// Captive Portal (RF06.1)
	// Captive Portal (RF06.1)
	var captiveAuth captive.Authenticator
	if db != nil {
		captiveAuth = captive.NewDBAuthenticator(db.Pool())
		fmt.Println("Captive Portal: autenticação via banco de dados")
	} else {
		captiveAuth = &captive.StaticCredentials{
			Users: map[string]captive.StaticUser{},
		}
		fmt.Println("Captive Portal: sem autenticação (banco indisponível)")
	}
	portal := captive.NewServer(cfg.Captive.Listen, identityResolver, captiveAuth,
		time.Duration(cfg.Captive.SessionTTL)*time.Hour)
	go func() {
		if err := portal.Start(); err != nil {
			fmt.Printf("Captive portal erro: %v\n", err)
		}
	}()

	// REST API (RF10.1-RF10.6)
	apiHandlers := api.NewHandlers(db, dnsCache, filterEngine, identityResolver, logPipeline, blacklist, whitelist, cfg.API.JWTSecret)
	apiRouter := api.NewRouter(apiHandlers)
	apiServer := &http.Server{Addr: cfg.API.Listen, Handler: apiRouter}

	go func() {
		fmt.Printf("API REST + Dashboard rodando em %s\n", cfg.API.Listen)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("API erro: %v\n", err)
		}
	}()

	// DNS Server
	blockIP := net.ParseIP(cfg.DNS.BlockIP)
	portalIP := net.ParseIP(cfg.DNS.PortalIP)

	resolver := dnsserver.NewResolver(cfg.DNS.Upstreams)
	handler := dnsserver.NewHandler(resolver, dnsCache, filterEngine, identityResolver, logPipeline, blockIP, portalIP)
	server := dnsserver.NewServer(cfg.DNS.Listen, handler)

	// Graceful shutdown (RNF02.4)
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		fmt.Printf("\nRecebido sinal %s, encerrando...\n", sig)

		stats := dnsCache.GetStats()
		fmt.Printf("Cache stats: %d hits, %d misses, %d entries\n",
			stats.Hits, stats.Misses, dnsCache.Size())
		fmt.Printf("Sessions ativas: %d\n", identityResolver.SessionCount())

		if logPipeline != nil {
			fmt.Printf("Log pipeline: %d pendentes\n", logPipeline.Pending())
			logPipeline.Stop()
		}

		portal.Shutdown()
		server.Shutdown()
	}()

	fmt.Printf("DNS Filter v1.0.0\n")
	fmt.Printf("DNS: %s\n", cfg.DNS.Listen)
	fmt.Printf("API: %s\n", cfg.API.Listen)
	fmt.Printf("Captive Portal: %s\n", cfg.Captive.Listen)
	fmt.Printf("Upstreams: %v\n", cfg.DNS.Upstreams)
	fmt.Printf("Cache: TTL floor=%ds, ceiling=%ds\n",
		cfg.Cache.TTLFloorSeconds, cfg.Cache.TTLCeilingSeconds)
	fmt.Printf("Blacklist: %d | Whitelist: %d\n", blacklist.Size(), whitelist.Size())
	fmt.Println("Pressione Ctrl+C para encerrar")

	if err := server.Start(); err != nil {
		fmt.Printf("Erro: %v\n", err)
		os.Exit(1)
	}
}

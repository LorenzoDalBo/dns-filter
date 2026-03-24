package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LorenzoDalBo/dns-filter/internal/cache"
	"github.com/LorenzoDalBo/dns-filter/internal/captive"
	dnsserver "github.com/LorenzoDalBo/dns-filter/internal/dns"
	"github.com/LorenzoDalBo/dns-filter/internal/filter"
	"github.com/LorenzoDalBo/dns-filter/internal/identity"
	"github.com/LorenzoDalBo/dns-filter/internal/store"
)

func main() {
	upstreams := []string{
		"8.8.8.8:53",
		"8.8.4.4:53",
		"1.1.1.1:53",
	}

	dnsCache := cache.New(30*time.Second, 1*time.Hour)
	blacklist := filter.NewBlacklist()
	whitelist := filter.NewBlacklist()

	// PostgreSQL connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://dnsfilter:dnsfilter123@localhost:5432/dnsfilter?sslmode=disable"
	}

	ctx := context.Background()
	db, err := store.New(ctx, dbURL)
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

	// Identity Resolver with default group=1
	identityResolver := identity.NewResolver(1)
	identityResolver.StartSessionEvictor()

	// Captive portal credentials (hardcoded for now, DB in future phase)
	creds := &captive.Credentials{
		Users: map[string]captive.UserInfo{
			"admin":     {Password: "admin123", UserID: 1, GroupID: 2},
			"visitante": {Password: "visit123", UserID: 2, GroupID: 4},
		},
	}

	// Captive portal HTTP server (RF06.1)
	portal := captive.NewServer(":8080", identityResolver, creds, 8*time.Hour)
	go func() {
		if err := portal.Start(); err != nil {
			fmt.Printf("Captive portal erro: %v\n", err)
		}
	}()

	blockIP := net.IPv4(0, 0, 0, 0)
	portalIP := net.IPv4(127, 0, 0, 1) // redirect to localhost for dev

	resolver := dnsserver.NewResolver(upstreams)
	handler := dnsserver.NewHandler(resolver, dnsCache, filterEngine, identityResolver, blockIP, portalIP)
	server := dnsserver.NewServer("127.0.0.1:5354", handler)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		fmt.Printf("\nRecebido sinal %s, encerrando...\n", sig)

		stats := dnsCache.GetStats()
		fmt.Printf("Cache stats: %d hits, %d misses, %d entries\n",
			stats.Hits, stats.Misses, dnsCache.Size())
		fmt.Printf("Sessions ativas: %d\n", identityResolver.SessionCount())

		portal.Shutdown()
		server.Shutdown()
	}()

	fmt.Println("Upstreams:", upstreams)
	fmt.Println("Cache: TTL floor=30s, ceiling=1h")
	fmt.Printf("Blacklist: %d | Whitelist: %d\n", blacklist.Size(), whitelist.Size())
	fmt.Println("Captive Portal: http://localhost:8080")
	fmt.Println("Pressione Ctrl+C para encerrar")

	if err := server.Start(); err != nil {
		fmt.Printf("Erro: %v\n", err)
		os.Exit(1)
	}
}

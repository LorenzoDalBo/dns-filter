package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LorenzoDalBo/dns-filter/internal/cache"
	dnsserver "github.com/LorenzoDalBo/dns-filter/internal/dns"
	"github.com/LorenzoDalBo/dns-filter/internal/filter"
)

func main() {
	upstreams := []string{
		"8.8.8.8:53",
		"8.8.4.4:53",
		"1.1.1.1:53",
	}

	dnsCache := cache.New(30*time.Second, 1*time.Hour)

	// Load blacklist from file if it exists (RF04.4)
	blacklist := filter.NewBlacklist()
	whitelist := filter.NewBlacklist()

	if _, err := os.Stat("blocklist.txt"); err == nil {
		count, err := blacklist.LoadFromFile("blocklist.txt")
		if err != nil {
			fmt.Printf("Aviso: erro ao carregar blocklist.txt: %v\n", err)
		} else {
			fmt.Printf("Blacklist: %d domínios carregados de blocklist.txt\n", count)
		}
	}

	if _, err := os.Stat("allowlist.txt"); err == nil {
		count, err := whitelist.LoadFromFile("allowlist.txt")
		if err != nil {
			fmt.Printf("Aviso: erro ao carregar allowlist.txt: %v\n", err)
		} else {
			fmt.Printf("Whitelist: %d domínios carregados de allowlist.txt\n", count)
		}
	}

	engine := filter.NewEngine(blacklist, whitelist)

	// IP returned for blocked domains (RF03.7)
	blockIP := net.IPv4(0, 0, 0, 0)

	resolver := dnsserver.NewResolver(upstreams)
	handler := dnsserver.NewHandler(resolver, dnsCache, engine, blockIP)
	server := dnsserver.NewServer("127.0.0.1:5354", handler)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		fmt.Printf("\nRecebido sinal %s, encerrando...\n", sig)

		stats := dnsCache.GetStats()
		fmt.Printf("Cache stats: %d hits, %d misses, %d entries\n",
			stats.Hits, stats.Misses, dnsCache.Size())

		server.Shutdown()
	}()

	fmt.Println("Upstreams:", upstreams)
	fmt.Println("Cache: TTL floor=30s, ceiling=1h")
	fmt.Printf("Blacklist: %d domínios | Whitelist: %d domínios\n",
		blacklist.Size(), whitelist.Size())
	fmt.Println("Pressione Ctrl+C para encerrar")

	if err := server.Start(); err != nil {
		fmt.Printf("Erro: %v\n", err)
		os.Exit(1)
	}
}

package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LorenzoDalBo/dns-filter/internal/cache"
	dnsserver "github.com/LorenzoDalBo/dns-filter/internal/dns"
)

func main() {
	upstreams := []string{
		"8.8.8.8:53",
		"8.8.4.4:53",
		"1.1.1.1:53",
	}

	// L1 in-memory cache with TTL floor=30s, ceiling=1h (RF02.3)
	dnsCache := cache.New(30*time.Second, 1*time.Hour)

	resolver := dnsserver.NewResolver(upstreams)
	handler := dnsserver.NewHandler(resolver, dnsCache)
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
	fmt.Println("Pressione Ctrl+C para encerrar")

	if err := server.Start(); err != nil {
		fmt.Printf("Erro: %v\n", err)
		os.Exit(1)
	}
}

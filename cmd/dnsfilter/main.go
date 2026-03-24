package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	dnsserver "github.com/LorenzoDalBo/dns-filter/internal/dns"
)

func main() {
	// Upstream resolvers with fallback order (RF01.4)
	upstreams := []string{
		"8.8.8.8:53",        // Google primary
		"8.8.4.4:53",        // Google secondary
		"1.1.1.1:53",        // Cloudflare
	}

	resolver := dnsserver.NewResolver(upstreams)
	handler := dnsserver.NewHandler(resolver)
	server := dnsserver.NewServer("127.0.0.1:5354", handler)

	// Graceful shutdown on Ctrl+C (RNF02.4)
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		fmt.Printf("\nRecebido sinal %s, encerrando...\n", sig)
		server.Shutdown()
	}()

	fmt.Println("Upstreams:", upstreams)
	fmt.Println("Pressione Ctrl+C para encerrar")

	if err := server.Start(); err != nil {
		fmt.Printf("Erro: %v\n", err)
		os.Exit(1)
	}
}
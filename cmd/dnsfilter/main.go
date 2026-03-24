package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/miekg/dns"
)

func main() {
	dns.HandleFunc(".", handleDNSQuery)

	port := "5354"
	addr := "127.0.0.1:" + port

	// Servidor UDP
	udpServer := &dns.Server{
		Addr: addr,
		Net:  "udp",
	}

	// Servidor TCP (nslookup no Windows frequentemente usa TCP)
	tcpServer := &dns.Server{
		Addr: addr,
		Net:  "tcp",
	}

	// Goroutine para graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		fmt.Printf("\nRecebido sinal %s, encerrando...\n", sig)
		udpServer.Shutdown()
		tcpServer.Shutdown()
	}()

	fmt.Printf("DNS Echo Server rodando em %s (UDP + TCP)\n", addr)
	fmt.Printf("Teste com: nslookup -port=%s google.com 127.0.0.1\n", port)
	fmt.Println("Pressione Ctrl+C para encerrar")

	// Roda UDP e TCP em paralelo usando goroutines
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := udpServer.ListenAndServe(); err != nil {
			log.Printf("Erro no servidor UDP: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := tcpServer.ListenAndServe(); err != nil {
			log.Printf("Erro no servidor TCP: %v", err)
		}
	}()

	// Espera ambos os servidores encerrarem
	wg.Wait()
}

func handleDNSQuery(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	for _, question := range r.Question {
		fmt.Printf("Query: %s (tipo %s) de %s\n",
			question.Name,
			dns.TypeToString[question.Qtype],
			w.RemoteAddr().String(),
		)

		if question.Qtype == dns.TypeA {
			rr := &dns.A{
				Hdr: dns.RR_Header{
					Name:   question.Name,
					Rrtype: dns.TypeA,
					Class:  dns.ClassINET,
					Ttl:    60,
				},
				A: net.IPv4(1, 2, 3, 4),
			}
			msg.Answer = append(msg.Answer, rr)
		}
	}

	if err := w.WriteMsg(msg); err != nil {
		fmt.Printf("Erro ao enviar resposta: %v\n", err)
	}
}

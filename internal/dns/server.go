package dns

import (
	"fmt"
	"sync"

	"github.com/miekg/dns"
)

// Server holds the UDP and TCP DNS listeners.
type Server struct {
	udp  *dns.Server
	tcp  *dns.Server
	addr string
}

func NewServer(addr string, handler *Handler) *Server {
	return &Server{
		addr: addr,
		udp: &dns.Server{
			Addr:    addr,
			Net:     "udp",
			Handler: handler,
		},
		tcp: &dns.Server{
			Addr:    addr,
			Net:     "tcp",
			Handler: handler,
		},
	}
}

// Start launches UDP and TCP listeners in parallel.
// Blocks until both servers stop.
func (s *Server) Start() error {
	fmt.Printf("DNS Server rodando em %s (UDP + TCP)\n", s.addr)

	var wg sync.WaitGroup
	errChan := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.udp.ListenAndServe(); err != nil {
			errChan <- fmt.Errorf("udp: %w", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.tcp.ListenAndServe(); err != nil {
			errChan <- fmt.Errorf("tcp: %w", err)
		}
	}()

	wg.Wait()

	// Check if any server returned an error
	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

// Shutdown gracefully stops both listeners (RNF02.4).
func (s *Server) Shutdown() {
	s.udp.Shutdown()
	s.tcp.Shutdown()
}

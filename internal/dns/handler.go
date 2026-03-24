package dns

import (
	"fmt"

	"github.com/miekg/dns"
)

// Handler orchestrates DNS query processing.
// In future phases, this is where Identity Resolver, Cache,
// and Policy Engine will plug in.
type Handler struct {
	resolver *Resolver
}

func NewHandler(resolver *Resolver) *Handler {
	return &Handler{resolver: resolver}
}

// ServeDNS is called by miekg/dns for every incoming query.
// It implements the dns.Handler interface.
func (h *Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	// Log the incoming query
	for _, q := range r.Question {
		fmt.Printf("Query: %s (tipo %s) de %s\n",
			q.Name,
			dns.TypeToString[q.Qtype],
			w.RemoteAddr().String(),
		)
	}

	// Forward to upstream and get real response
	resp, err := h.resolver.Resolve(r)
	if err != nil {
		fmt.Printf("Erro no upstream: %v\n", err)
		// If all upstreams fail, return SERVFAIL (RNF02.3)
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(msg)
		return
	}

	// Send the real response back to the client
	resp.SetReply(r)
	if err := w.WriteMsg(resp); err != nil {
		fmt.Printf("Erro ao enviar resposta: %v\n", err)
	}
}

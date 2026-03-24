package dns

import (
	"fmt"

	"github.com/LorenzoDalBo/dns-filter/internal/cache"
	"github.com/miekg/dns"
)

// Handler orchestrates DNS query processing.
// Current pipeline: Cache → Resolver
// Future phases add: Identity Resolver → Cache → Policy Engine → Resolver
type Handler struct {
	resolver *Resolver
	cache    *cache.Cache
}

func NewHandler(resolver *Resolver, cache *cache.Cache) *Handler {
	return &Handler{
		resolver: resolver,
		cache:    cache,
	}
}

func (h *Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		return
	}

	question := r.Question[0]
	qName := question.Name
	qType := question.Qtype

	fmt.Printf("Query: %s (tipo %s) de %s",
		qName,
		dns.TypeToString[qType],
		w.RemoteAddr().String(),
	)

	// Step 1: Check cache (RF02.1)
	if cached := h.cache.Get(qName, qType); cached != nil {
		fmt.Printf(" [CACHE HIT]\n")
		cached.SetReply(r)
		if err := w.WriteMsg(cached); err != nil {
			fmt.Printf("Erro ao enviar resposta do cache: %v\n", err)
		}
		return
	}

	fmt.Printf(" [CACHE MISS]\n")

	// Step 2: Forward to upstream
	resp, err := h.resolver.Resolve(r)
	if err != nil {
		fmt.Printf("Erro no upstream: %v\n", err)
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(msg)
		return
	}

	// Step 3: Store in cache for next time
	h.cache.Set(qName, qType, resp)

	// Step 4: Send response to client
	resp.SetReply(r)
	if err := w.WriteMsg(resp); err != nil {
		fmt.Printf("Erro ao enviar resposta: %v\n", err)
	}
}

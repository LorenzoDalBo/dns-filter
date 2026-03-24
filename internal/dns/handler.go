package dns

import (
	"fmt"
	"net"

	"github.com/LorenzoDalBo/dns-filter/internal/cache"
	"github.com/LorenzoDalBo/dns-filter/internal/filter"
	"github.com/miekg/dns"
)

// Handler orchestrates DNS query processing.
// Pipeline: Cache check → Policy Engine (always) → Upstream (on miss)
type Handler struct {
	resolver *Resolver
	cache    *cache.Cache
	filter   *filter.Engine
	blockIP  net.IP // IP returned for blocked domains (RF03.7)
}

func NewHandler(resolver *Resolver, cache *cache.Cache, filter *filter.Engine, blockIP net.IP) *Handler {
	return &Handler{
		resolver: resolver,
		cache:    cache,
		filter:   filter,
		blockIP:  blockIP,
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

	// Step 1: Policy Engine — ALWAYS runs, even before cache (RF03.1)
	result := h.filter.Evaluate(qName)
	if result.Action == filter.ActionBlock {
		fmt.Printf(" [BLOCKED: %s]\n", result.Reason)
		h.sendBlockResponse(w, r, qName, qType)
		return
	}

	// Step 2: Check cache (RF02.1)
	if cached := h.cache.Get(qName, qType); cached != nil {
		fmt.Printf(" [CACHE HIT]\n")
		cached.SetReply(r)
		if err := w.WriteMsg(cached); err != nil {
			fmt.Printf("Erro ao enviar resposta do cache: %v\n", err)
		}
		return
	}

	fmt.Printf(" [CACHE MISS]\n")

	// Step 3: Forward to upstream
	resp, err := h.resolver.Resolve(r)
	if err != nil {
		fmt.Printf("Erro no upstream: %v\n", err)
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(msg)
		return
	}

	// Step 4: Store in cache
	h.cache.Set(qName, qType, resp)

	// Step 5: Send response
	resp.SetReply(r)
	if err := w.WriteMsg(resp); err != nil {
		fmt.Printf("Erro ao enviar resposta: %v\n", err)
	}
}

// sendBlockResponse returns 0.0.0.0 for A queries or empty NOERROR for others.
func (h *Handler) sendBlockResponse(w dns.ResponseWriter, r *dns.Msg, qName string, qType uint16) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	// Only return the block IP for A queries (RF03.7)
	if qType == dns.TypeA {
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   qName,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    60,
			},
			A: h.blockIP,
		})
	}
	// For AAAA, CNAME, etc: return NOERROR with empty answer

	if err := w.WriteMsg(msg); err != nil {
		fmt.Printf("Erro ao enviar block response: %v\n", err)
	}
}

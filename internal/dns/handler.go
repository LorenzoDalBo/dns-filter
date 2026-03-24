package dns

import (
	"fmt"
	"net"

	"github.com/LorenzoDalBo/dns-filter/internal/cache"
	"github.com/LorenzoDalBo/dns-filter/internal/filter"
	"github.com/LorenzoDalBo/dns-filter/internal/identity"
	"github.com/miekg/dns"
)

// Handler orchestrates DNS query processing.
// Pipeline: Identity → Policy Engine → Cache → Upstream
type Handler struct {
	resolver *Resolver
	cache    *cache.Cache
	filter   *filter.Engine
	identity *identity.Resolver
	blockIP  net.IP // IP returned for blocked domains (RF03.7)
	portalIP net.IP // IP returned to redirect to captive portal (RF06.2)
}

func NewHandler(resolver *Resolver, cache *cache.Cache, filter *filter.Engine, identityResolver *identity.Resolver, blockIP net.IP, portalIP net.IP) *Handler {
	return &Handler{
		resolver: resolver,
		cache:    cache,
		filter:   filter,
		identity: identityResolver,
		blockIP:  blockIP,
		portalIP: portalIP,
	}
}

func (h *Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		return
	}

	question := r.Question[0]
	qName := question.Name
	qType := question.Qtype

	// Extract client IP
	clientIP := extractIP(w.RemoteAddr())

	fmt.Printf("Query: %s (tipo %s) de %s",
		qName,
		dns.TypeToString[qType],
		clientIP,
	)

	// Step 1: Identity Resolution (RF05.1-RF05.8)
	id, err := h.identity.Resolve(clientIP)
	if err != nil {
		fmt.Printf(" [IDENTITY ERROR: %v]\n", err)
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(msg)
		return
	}

	// Step 2: Captive Portal redirect (RF06.2)
	if id.AuthMode == identity.AuthCaptivePortal {
		fmt.Printf(" [CAPTIVE REDIRECT]\n")
		h.sendCaptiveResponse(w, r, qName, qType)
		return
	}

	// Step 3: Policy Engine — ALWAYS runs (RF03.1)
	result := h.filter.Evaluate(qName)
	if result.Action == filter.ActionBlock {
		fmt.Printf(" [BLOCKED: %s]\n", result.Reason)
		h.sendBlockResponse(w, r, qName, qType)
		return
	}

	// Step 4: Check cache (RF02.1)
	if cached := h.cache.Get(qName, qType); cached != nil {
		fmt.Printf(" [CACHE HIT]\n")
		cached.SetReply(r)
		if err := w.WriteMsg(cached); err != nil {
			fmt.Printf("Erro ao enviar resposta do cache: %v\n", err)
		}
		return
	}

	fmt.Printf(" [CACHE MISS]\n")

	// Step 5: Forward to upstream
	resp, err := h.resolver.Resolve(r)
	if err != nil {
		fmt.Printf("Erro no upstream: %v\n", err)
		msg := new(dns.Msg)
		msg.SetRcode(r, dns.RcodeServerFailure)
		w.WriteMsg(msg)
		return
	}

	// Step 6: Store in cache
	h.cache.Set(qName, qType, resp)

	// Step 7: Send response
	resp.SetReply(r)
	if err := w.WriteMsg(resp); err != nil {
		fmt.Printf("Erro ao enviar resposta: %v\n", err)
	}
}

// sendBlockResponse returns 0.0.0.0 for blocked A queries.
func (h *Handler) sendBlockResponse(w dns.ResponseWriter, r *dns.Msg, qName string, qType uint16) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

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

	if err := w.WriteMsg(msg); err != nil {
		fmt.Printf("Erro ao enviar block response: %v\n", err)
	}
}

// sendCaptiveResponse returns the portal IP so the browser lands on the login page (RF06.2).
func (h *Handler) sendCaptiveResponse(w dns.ResponseWriter, r *dns.Msg, qName string, qType uint16) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	if qType == dns.TypeA {
		msg.Answer = append(msg.Answer, &dns.A{
			Hdr: dns.RR_Header{
				Name:   qName,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    10, // short TTL so it refreshes after login
			},
			A: h.portalIP,
		})
	}

	if err := w.WriteMsg(msg); err != nil {
		fmt.Printf("Erro ao enviar captive response: %v\n", err)
	}
}

// extractIP gets the IP from a net.Addr (strips the port).
func extractIP(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.UDPAddr:
		return v.IP
	case *net.TCPAddr:
		return v.IP
	default:
		host, _, err := net.SplitHostPort(addr.String())
		if err != nil {
			return nil
		}
		return net.ParseIP(host)
	}
}

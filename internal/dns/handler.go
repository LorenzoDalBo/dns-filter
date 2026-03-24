package dns

import (
	"fmt"
	"net"
	"time"

	"github.com/LorenzoDalBo/dns-filter/internal/cache"
	"github.com/LorenzoDalBo/dns-filter/internal/filter"
	"github.com/LorenzoDalBo/dns-filter/internal/identity"
	"github.com/LorenzoDalBo/dns-filter/internal/logging"
	"github.com/miekg/dns"
)

// Handler orchestrates DNS query processing.
// Pipeline: Identity → Policy Engine → Cache → Upstream → Log
type Handler struct {
	resolver *Resolver
	cache    *cache.Cache
	filter   *filter.Engine
	identity *identity.Resolver
	logger   *logging.Pipeline
	blockIP  net.IP
	portalIP net.IP
}

func NewHandler(resolver *Resolver, cache *cache.Cache, filter *filter.Engine, identityResolver *identity.Resolver, logger *logging.Pipeline, blockIP net.IP, portalIP net.IP) *Handler {
	return &Handler{
		resolver: resolver,
		cache:    cache,
		filter:   filter,
		identity: identityResolver,
		logger:   logger,
		blockIP:  blockIP,
		portalIP: portalIP,
	}
}

func (h *Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		return
	}

	start := time.Now()

	question := r.Question[0]
	qName := question.Name
	qType := question.Qtype

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
		h.log(start, clientIP, id, qName, qType, logging.ActionBlocked, logging.BlockReasonPolicy, nil, "captive")
		return
	}

	// Step 3: Policy Engine — ALWAYS runs (RF03.1)
	result := h.filter.Evaluate(qName)
	if result.Action == filter.ActionBlock {
		fmt.Printf(" [BLOCKED: %s]\n", result.Reason)
		h.sendBlockResponse(w, r, qName, qType)
		h.log(start, clientIP, id, qName, qType, logging.ActionBlocked, logging.BlockReasonBlacklist, nil, "blocked")
		return
	}

	// Step 4: Check cache (RF02.1)
	if cached := h.cache.Get(qName, qType); cached != nil {
		fmt.Printf(" [CACHE HIT]\n")
		cached.SetReply(r)
		if err := w.WriteMsg(cached); err != nil {
			fmt.Printf("Erro ao enviar resposta do cache: %v\n", err)
		}
		responseIP := extractAnswerIP(cached)
		h.log(start, clientIP, id, qName, qType, logging.ActionCached, logging.BlockReasonNone, responseIP, "cache")
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

	responseIP := extractAnswerIP(resp)
	h.log(start, clientIP, id, qName, qType, logging.ActionAllowed, logging.BlockReasonNone, responseIP, "upstream")
}

// log sends a query event to the async pipeline (RF07.1, RF07.2).
func (h *Handler) log(start time.Time, clientIP net.IP, id *identity.Identity, qName string, qType uint16, action logging.Action, reason logging.BlockReason, responseIP net.IP, upstream string) {
	if h.logger == nil {
		return
	}

	entry := logging.Entry{
		QueriedAt:   start,
		ClientIP:    clientIP,
		GroupID:     id.GroupID,
		Domain:      qName,
		QueryType:   qType,
		Action:      action,
		BlockReason: reason,
		ResponseIP:  responseIP,
		ResponseMs:  float32(time.Since(start).Seconds() * 1000),
		Upstream:    upstream,
	}

	if id.UserID != 0 {
		uid := id.UserID
		entry.UserID = &uid
	}

	h.logger.Send(entry)
}

// extractAnswerIP gets the first A record IP from a response.
func extractAnswerIP(msg *dns.Msg) net.IP {
	for _, rr := range msg.Answer {
		if a, ok := rr.(*dns.A); ok {
			return a.A
		}
	}
	return nil
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
				Ttl:    10,
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

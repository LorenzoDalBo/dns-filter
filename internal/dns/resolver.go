package dns

import (
	"fmt"
	"time"

	mdns "github.com/miekg/dns"
)

// Resolver encaminha queries DNS para servidores upstream.
// Suporta múltiplos upstreams com fallback automático (RF01.4).
type Resolver struct {
	upstreams []string
	client    *mdns.Client
}

// NewResolver cria um Resolver com a lista de upstreams fornecida.
// Cada upstream deve incluir porta (ex: "8.8.8.8:53").
func NewResolver(upstreams []string) *Resolver {
	return &Resolver{
		upstreams: upstreams,
		client: &mdns.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Resolve encaminha a query para os upstreams em ordem.
// Se o primeiro falhar, tenta o próximo (fallback).
// Retorna a resposta do primeiro upstream que responder.
func (r *Resolver) Resolve(req *mdns.Msg) (*mdns.Msg, error) {
	for _, upstream := range r.upstreams {
		resp, _, err := r.client.Exchange(req, upstream)
		if err != nil {
			fmt.Printf("  Upstream %s falhou: %v, tentando próximo...\n", upstream, err)
			continue
		}
		return resp, nil
	}

	return nil, fmt.Errorf("resolver: todos os upstreams falharam")
}

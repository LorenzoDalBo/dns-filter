package main

import (
	"testing"

	"github.com/miekg/dns"
)

func TestForwarder(t *testing.T) {
	// Server must be running on 127.0.0.1:5354

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("google.com"), dns.TypeA)

	client := new(dns.Client)
	resp, _, err := client.Exchange(msg, "127.0.0.1:5354")
	if err != nil {
		t.Fatalf("Erro ao consultar servidor: %v", err)
	}

	if len(resp.Answer) == 0 {
		t.Fatal("Resposta sem answers — upstream pode estar inacessível")
	}

	// Now we expect a REAL IP from Google, not 1.2.3.4
	for _, answer := range resp.Answer {
		t.Logf("Resposta: %s", answer.String())
	}
}

func TestForwarderMultipleTypes(t *testing.T) {
	tests := []struct {
		domain    string
		queryType uint16
		typeName  string
	}{
		{"google.com", dns.TypeA, "A"},
		{"google.com", dns.TypeAAAA, "AAAA"},
		{"gmail.com", dns.TypeMX, "MX"},
	}

	client := new(dns.Client)

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			msg := new(dns.Msg)
			msg.SetQuestion(dns.Fqdn(tt.domain), tt.queryType)

			resp, _, err := client.Exchange(msg, "127.0.0.1:5354")
			if err != nil {
				t.Fatalf("Erro na query %s: %v", tt.typeName, err)
			}

			t.Logf("%s %s: %d answers", tt.domain, tt.typeName, len(resp.Answer))
			for _, answer := range resp.Answer {
				t.Logf("  %s", answer.String())
			}
		})
	}
}

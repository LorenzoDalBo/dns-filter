package main

import (
	"testing"

	"github.com/miekg/dns"
)

func queryServer(domain string, qtype uint16) (*dns.Msg, error) {
	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn(domain), qtype)
	client := new(dns.Client)
	resp, _, err := client.Exchange(msg, "127.0.0.1:5354")
	return resp, err
}

func TestAllowedDomain(t *testing.T) {
	resp, err := queryServer("google.com", dns.TypeA)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(resp.Answer) == 0 {
		t.Fatal("Expected answers for allowed domain")
	}

	a := resp.Answer[0].(*dns.A)
	if a.A.String() == "0.0.0.0" {
		t.Error("Allowed domain should not return 0.0.0.0")
	}
	t.Logf("google.com → %s (allowed)", a.A.String())
}

func TestBlockedDomain(t *testing.T) {
	resp, err := queryServer("ads.google.com", dns.TypeA)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(resp.Answer) == 0 {
		t.Fatal("Expected answer with block IP")
	}

	a := resp.Answer[0].(*dns.A)
	if a.A.String() != "0.0.0.0" {
		t.Errorf("Blocked domain should return 0.0.0.0, got %s", a.A.String())
	}
	t.Logf("ads.google.com → %s (blocked)", a.A.String())
}

func TestBlockedSubdomain(t *testing.T) {
	// doubleclick.net is in blocklist, so sub.doubleclick.net should be blocked too
	resp, err := queryServer("tracker.doubleclick.net", dns.TypeA)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(resp.Answer) == 0 {
		t.Fatal("Expected answer with block IP")
	}

	a := resp.Answer[0].(*dns.A)
	if a.A.String() != "0.0.0.0" {
		t.Errorf("Subdomain of blocked domain should return 0.0.0.0, got %s", a.A.String())
	}
	t.Logf("tracker.doubleclick.net → %s (blocked via wildcard)", a.A.String())
}

func TestWhitelistOverridesBlacklist(t *testing.T) {
	// safe.doubleclick.net is in allowlist, even though doubleclick.net is blocked.
	// We verify it was NOT blocked (no 0.0.0.0 response).
	// Since the domain doesn't exist in real DNS, upstream returns empty answer,
	// which is correct — the important thing is it wasn't blocked.
	resp, err := queryServer("safe.doubleclick.net", dns.TypeA)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// If it were blocked, we'd get 0.0.0.0 as answer
	for _, answer := range resp.Answer {
		if a, ok := answer.(*dns.A); ok && a.A.String() == "0.0.0.0" {
			t.Error("Whitelisted domain should NOT be blocked (RF03.5)")
		}
	}

	// NOERROR means the query went through to upstream (not blocked)
	if resp.Rcode != dns.RcodeSuccess {
		t.Errorf("Expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}

	t.Logf("safe.doubleclick.net → not blocked, %d answers (whitelist override, RF03.5)", len(resp.Answer))
}
func TestBlockedAAAAReturnsEmpty(t *testing.T) {
	// AAAA query for blocked domain should return NOERROR with no answer
	resp, err := queryServer("ads.google.com", dns.TypeAAAA)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(resp.Answer) != 0 {
		t.Errorf("Blocked AAAA should return empty answer, got %d answers", len(resp.Answer))
	}
	if resp.Rcode != dns.RcodeSuccess {
		t.Errorf("Blocked AAAA should return NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}
	t.Log("ads.google.com AAAA → empty NOERROR (blocked, non-A type)")
}

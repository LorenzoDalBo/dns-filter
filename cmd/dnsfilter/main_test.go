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
	resp, err := queryServer("safe.doubleclick.net", dns.TypeA)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	for _, answer := range resp.Answer {
		if a, ok := answer.(*dns.A); ok && a.A.String() == "0.0.0.0" {
			t.Error("Whitelisted domain should NOT be blocked (RF03.5)")
		}
	}

	if resp.Rcode != dns.RcodeSuccess {
		t.Errorf("Expected NOERROR, got %s", dns.RcodeToString[resp.Rcode])
	}

	t.Logf("safe.doubleclick.net → not blocked, %d answers (whitelist override, RF03.5)", len(resp.Answer))
}

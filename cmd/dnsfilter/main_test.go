package main

import (
	"testing"

	"github.com/miekg/dns"
)

func TestEchoServer(t *testing.T) {
	// O servidor precisa estar rodando em outro terminal
	// Este teste age como um cliente DNS

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("google.com"), dns.TypeA)

	client := new(dns.Client)
	resp, _, err := client.Exchange(msg, "127.0.0.1:5354")
	if err != nil {
		t.Fatalf("Erro ao consultar servidor: %v", err)
	}

	if len(resp.Answer) == 0 {
		t.Fatal("Resposta sem answers")
	}

	answer, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatal("Resposta não é tipo A")
	}

	expected := "1.2.3.4"
	if answer.A.String() != expected {
		t.Errorf("IP esperado %s, recebido %s", expected, answer.A.String())
	}

	t.Logf("Resposta recebida: %s -> %s", answer.Hdr.Name, answer.A.String())
}

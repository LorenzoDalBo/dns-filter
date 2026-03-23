# DNS Filter

Filtro DNS empresarial para redes de 500 a 5000 dispositivos. Construído do zero em Go.

## Funcionalidades

- Servidor DNS completo (UDP/TCP, porta 53) com cache multi-camada
- Motor de filtragem com blacklists, whitelists e categorias por grupo
- Identificação de clientes por IP/CIDR com Captive Portal
- Dashboard administrativo com logs filtráveis em tempo real
- Pipeline de log assíncrono de alta performance
- API REST completa para gerenciamento

## Stack

| Camada | Tecnologia |
|--------|-----------|
| DNS Server | Go + miekg/dns |
| Banco de dados | PostgreSQL + TimescaleDB |
| Cache | In-memory (L1) + Redis (L2) |
| Frontend | React + Vite + TanStack Table + Shadcn/ui |
| Deploy | Binário único, on-premise |

## Início Rápido

### Pré-requisitos

- Go 1.24+
- Docker e Docker Compose
- Node.js 20+ (para o frontend)

### Desenvolvimento

```bash
# Subir dependências (PostgreSQL + Redis)
docker-compose up -d

# Compilar e rodar
go run ./cmd/dnsfilter

# Rodar testes
go test ./...
```

### Testar o servidor DNS

```bash
nslookup google.com 127.0.0.1
```

## Estrutura do Projeto

```
cmd/dnsfilter/       → Entry point do binário
internal/dns/        → Servidor DNS (listener, handler, upstream)
internal/filter/     → Motor de filtragem (blacklist, whitelist, políticas)
internal/cache/      → Cache DNS (L1 memória + L2 Redis)
internal/identity/   → Identificação de clientes (IP/CIDR, sessões)
internal/captive/    → Captive Portal (página de login HTTP)
internal/logging/    → Pipeline assíncrono de logs
internal/api/        → API REST + autenticação JWT
internal/config/     → Configuração YAML
internal/store/      → Camada de persistência PostgreSQL
web/                 → Frontend React
migrations/          → Migrations SQL
docs/                → Documentação detalhada
```

## Documentação

- [Requisitos Completos](docs/REQUIREMENTS.md) — RF01-RF10 e RNF01-RNF07

## Licença

Este projeto está em desenvolvimento.

# DNS Filter

Filtro DNS empresarial para redes de 500 a 5000 dispositivos. Construído do zero em Go.

## Funcionalidades

- Servidor DNS completo (UDP/TCP, porta 53) com cache multi-camada
- Motor de filtragem com blacklists, whitelists e wildcard blocking
- Identificação de clientes por IP/CIDR com Captive Portal
- Dashboard administrativo com logs filtráveis em tempo real
- Pipeline de log assíncrono de alta performance
- API REST completa com autenticação JWT
- Rate limiting para prevenção de brute force
- Configuração via arquivo YAML
- Auto-migrate do banco de dados
- Binário único com dashboard embarcado

## Stack

| Camada | Tecnologia |
|--------|------------|
| DNS Server | Go + miekg/dns |
| Banco de dados | PostgreSQL + TimescaleDB |
| Cache | In-memory (L1) |
| Frontend | React + Vite + TanStack Table + Tailwind CSS |
| Deploy | Binário único ou Docker Compose |

## Início Rápido

### Opção 1: Docker Compose (recomendado)
```bash
# DNS Filter Empresarial

Filtro DNS empresarial para redes de 500 a 5000 dispositivos. Construído do zero em Go com a biblioteca [miekg/dns](https://github.com/miekg/dns).

Bloqueia domínios de publicidade, malware e rastreamento em toda a rede, com dashboard administrativo em tempo real, sem precisar instalar nada nos dispositivos.

## Funcionalidades

- **Servidor DNS completo** (UDP/TCP, porta 53) com cache multi-camada (L1 in-memory + L2 Redis)
- **Motor de filtragem** com blacklists, whitelists, categorias e políticas por grupo
- **88.000+ domínios** bloqueáveis via listas externas (StevenBlack, AdGuard, etc.)
- **Dashboard administrativo** com logs filtráveis em tempo real, métricas e gráficos
- **Identificação de clientes** por IP/CIDR com Captive Portal para autenticação
- **Pipeline de log assíncrono** de alta performance com retenção configurável
- **API REST completa** com autenticação JWT e rate limiting
- **Binário único** com dashboard React embarcado — zero dependências em runtime
- **Auto-migrate** do banco de dados na primeira execução
- **Download automático** de listas externas com atualização periódica

## Stack Técnica

| Camada | Tecnologia |
|--------|------------|
| DNS Server | Go + miekg/dns |
| Banco de dados | PostgreSQL + TimescaleDB |
| Cache L1 | In-memory (hashmap) |
| Cache L2 | Redis (opcional) |
| Frontend | React + Vite + TanStack Table + Tailwind CSS |
| Deploy | Binário único ou Docker Compose |

---

## Instalação

### Pré-requisitos

- **Go 1.24+** (para compilar)
- **Docker + Docker Compose** (para PostgreSQL e Redis)
- **Node.js 20+** (apenas para desenvolvimento do frontend)

### 1. Clone o repositório

```bash
git clone https://github.com/LorenzoDalBo/dns-filter.git
cd dns-filter
```

### 2. Suba o PostgreSQL e Redis

```bash
docker-compose up -d postgres redis
```

Isso cria dois containers: `dnsfilter-postgres` (porta 5432) e `dnsfilter-redis` (porta 6379).

### 3. Configure o servidor

Copie o arquivo de configuração e ajuste para sua rede:

```bash
cp configs/dnsfilter.yaml configs/minha-config.yaml
```

Edite `configs/minha-config.yaml`:

```yaml
dns:
  # IP real do servidor na rede local
  listen: "192.168.1.100:53"
  upstreams:
    - "8.8.8.8:53"
    - "8.8.4.4:53"
    - "1.1.1.1:53"
  block_ip: "0.0.0.0"
  # IP do servidor (para redirect do Captive Portal)
  portal_ip: "192.168.1.100"

cache:
  ttl_floor_seconds: 30
  ttl_ceiling_seconds: 3600

api:
  listen: ":8081"
  # IMPORTANTE: gere um secret aleatório de 32+ caracteres
  jwt_secret: "gere-um-secret-aleatorio-aqui-com-32-chars"
  # TLS (opcional): descomente para habilitar HTTPS
  # tls_cert: "/etc/dnsfilter/cert.pem"
  # tls_key: "/etc/dnsfilter/key.pem"

captive:
  listen: ":80"
  session_ttl_hours: 8

database:
  url: "postgres://dnsfilter:dnsfilter123@localhost:5432/dnsfilter?sslmode=disable"
  retention_days: 120
  log_buffer_size: 100000

redis:
  addr: "localhost:6379"

log:
  level: "info"
```

> **Importante:** Troque `192.168.1.100` pelo IP real do servidor na sua rede. Use `ipconfig` (Windows) ou `ip addr` (Linux) para descobrir.

### 4. Compile e execute

```bash
go build -o dnsfilter.exe ./cmd/dnsfilter
./dnsfilter.exe -config configs/minha-config.yaml
```

> **Windows:** A porta 53 requer terminal como Administrador.
> **Linux:** Use `sudo` ou `setcap cap_net_bind_service=+ep dnsfilter`.

Na primeira execução, o auto-migrate cria todas as tabelas automaticamente. O usuário admin padrão é criado com credenciais `admin` / `admin123`.

### 5. Teste o funcionamento

Em outro terminal:

```bash
# Deve resolver normalmente
nslookup google.com SEU_IP

# Deve retornar 0.0.0.0 (bloqueado)
nslookup ads.google.com SEU_IP
```

### 6. Acesse o dashboard

Abra no browser: `http://SEU_IP:8081`

Faça login com `admin` / `admin123`.

> **IMPORTANTE:** Troque a senha padrão imediatamente em produção.

---

## Configuração de Rede

### Opção A: Apontar o roteador (recomendado)

Configure o roteador para usar o IP do servidor como DNS primário. Assim todos os dispositivos da rede usam o filtro automaticamente.

**MikroTik:**
```
/ip dns set servers=192.168.1.100
/ip dns set allow-remote-requests=yes
```

**Roteadores comuns:** Acesse a interface web do roteador, procure configurações de DHCP/DNS e troque o DNS primário para o IP do servidor.

### Opção B: Configurar por dispositivo

Em cada dispositivo, altere o DNS manualmente para o IP do servidor.

**Windows (temporário):**
```cmd
netsh interface ip set dns "Wi-Fi" static 192.168.1.100
```

Para reverter:
```cmd
netsh interface ip set dns "Wi-Fi" dhcp
```

**Android:** Configurações → Wi-Fi → Editar rede → IP estático → DNS 1: IP do servidor

**Linux:**
```bash
sudo resolvectl dns eth0 192.168.1.100
```

### Dica importante: DNS privado no Android

O Android 9+ tem "DNS privado" (DNS-over-HTTPS) que ignora o DNS da rede. Para o filtro funcionar em celulares Android, desative em:

**Configurações → Conexões → Mais configurações de conexão → DNS privado → Desativado**

---

## Gerenciamento de Listas de Bloqueio

### Via Dashboard (Interface Web)

Acesse `http://SEU_IP:8081`, faça login, e vá em **Listas de Bloqueio**.

#### Adicionar lista externa (por URL)

1. Preencha o **Nome** (ex: "StevenBlack Ads+Malware")
2. Cole a **URL** da lista (ex: `https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts`)
3. Selecione o **Tipo**: Blacklist (bloquear) ou Whitelist (permitir)
4. Clique **Criar**
5. Clique **Baixar Listas Externas** para fazer o download imediato

#### Listas recomendadas

| Nome | URL | Domínios | Descrição |
|------|-----|----------|-----------|
| StevenBlack Unified | `https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts` | ~88.000 | Ads, malware, fakenews |
| AdGuard DNS | `https://v.firebog.net/hosts/AdguardDNS.txt` | ~45.000 | Publicidade e rastreamento |
| Malware Domains | `https://v.firebog.net/hosts/Prigent-Malware.txt` | ~30.000 | Malware e phishing |
| Easy Privacy | `https://v.firebog.net/hosts/Easyprivacy.txt` | ~15.000 | Rastreamento |

#### Recarregar listas em memória

Após criar ou modificar listas, clique **Recarregar Listas** para aplicar as mudanças sem reiniciar o servidor.

### Via API REST

#### Criar lista

```bash
# Obter token JWT
TOKEN=$(curl -s -X POST http://SEU_IP:8081/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r '.token')

# Criar lista externa
curl -X POST http://SEU_IP:8081/api/lists \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"StevenBlack","source_url":"https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts","list_type":0}'

# Disparar download
curl -X POST http://SEU_IP:8081/api/lists/download \
  -H "Authorization: Bearer $TOKEN"

# Recarregar listas em memória
curl -X POST http://SEU_IP:8081/api/lists/reload \
  -H "Authorization: Bearer $TOKEN"
```

#### Adicionar domínios individuais

```bash
# Criar lista manual
curl -X POST http://SEU_IP:8081/api/lists \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Bloqueios manuais","source_url":"","list_type":0}'

# Adicionar domínios (use o ID retornado na criação)
curl -X POST http://SEU_IP:8081/api/lists/1/entries \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"domains":["facebook.com","instagram.com","tiktok.com"]}'

# Recarregar
curl -X POST http://SEU_IP:8081/api/lists/reload \
  -H "Authorization: Bearer $TOKEN"
```

---

## Dashboard

### Visão Geral

A tela principal mostra métricas em tempo real (atualiza a cada 5 segundos):

- Queries por segundo e latência média
- Cache hit rate
- Total de queries (hoje/semana/mês)
- Percentual de bloqueio
- Top 10 domínios consultados
- Top 10 domínios bloqueados
- Top 10 clientes por volume

### Logs

A tela de logs permite filtrar por:

- Domínio (busca parcial)
- IP do cliente
- Ação (permitido/bloqueado/cache)
- Data e horário (range)

Paginação server-side — funciona mesmo com milhões de registros.

### Gerenciamento

- **Usuários:** Criar, editar e desativar usuários do dashboard (admin e viewer)
- **Grupos:** Criar grupos com políticas de bloqueio por categoria
- **Listas:** Criar, baixar e recarregar listas de bloqueio
- **Ranges:** Associar faixas de IP a grupos

---

## Grupos e Políticas por Categoria

### Como funciona

1. **Categorias** são pré-definidas: malware, ads, adult, social, streaming, gaming
2. **Listas** podem ser associadas a categorias
3. **Grupos** têm políticas que definem quais categorias bloquear
4. Cada **faixa de IP** é associada a um grupo

### Exemplo: bloquear redes sociais para o grupo "Funcionários"

```bash
# 1. Criar grupo
curl -X POST http://SEU_IP:8081/api/groups \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Funcionarios","description":"Bloqueia redes sociais"}'

# 2. Associar lista a uma categoria (ex: categoria 4 = social)
curl -X PUT http://SEU_IP:8081/api/lists/1/categories \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"categories":[4]}'

# 3. Definir política do grupo (bloquear categoria 4)
curl -X PUT http://SEU_IP:8081/api/groups/2/policy \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"categories":[4]}'

# 4. Associar range de IPs ao grupo
curl -X POST http://SEU_IP:8081/api/ranges \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"cidr":"192.168.1.0/24","group_id":2,"auth_mode":0,"description":"Rede dos funcionários"}'
```

---

## API REST — Referência Completa

### Autenticação

| Método | Endpoint | Auth | Descrição |
|--------|----------|------|-----------|
| POST | `/api/auth/login` | Não | Login (retorna JWT) |
| POST | `/api/auth/refresh` | JWT | Renovar token |

### Dashboard e Métricas

| Método | Endpoint | Auth | Descrição |
|--------|----------|------|-----------|
| GET | `/health` | Não | Health check |
| GET | `/api/dashboard` | JWT | Estatísticas do dashboard |
| GET | `/api/metrics` | JWT | Métricas operacionais |
| GET | `/api/logs` | JWT | Logs DNS paginados |

#### Parâmetros de `/api/logs`

| Param | Tipo | Descrição |
|-------|------|-----------|
| `domain` | string | Filtro por domínio (busca parcial) |
| `client_ip` | string | Filtro por IP do cliente |
| `action` | string | "0" (permitido), "1" (bloqueado), "2" (cache) |
| `date_from` | datetime | Data/hora inicial |
| `date_to` | datetime | Data/hora final |
| `limit` | int | Resultados por página (default: 50, max: 100) |
| `offset` | int | Offset para paginação |

### Usuários

| Método | Endpoint | Auth | Descrição |
|--------|----------|------|-----------|
| GET | `/api/users` | Admin | Listar usuários |
| POST | `/api/users` | Admin | Criar usuário |
| PUT | `/api/users/{id}` | Admin | Editar usuário |
| DELETE | `/api/users/{id}` | Admin | Desativar usuário |

### Grupos e Políticas

| Método | Endpoint | Auth | Descrição |
|--------|----------|------|-----------|
| GET | `/api/groups` | Admin | Listar grupos |
| POST | `/api/groups` | Admin | Criar grupo |
| PUT | `/api/groups/{id}` | Admin | Editar grupo |
| DELETE | `/api/groups/{id}` | Admin | Remover grupo |
| GET | `/api/groups/{id}/policy` | Admin | Ver política do grupo |
| PUT | `/api/groups/{id}/policy` | Admin | Definir categorias bloqueadas |
| GET | `/api/categories` | Admin | Listar categorias disponíveis |

### Listas de Bloqueio

| Método | Endpoint | Auth | Descrição |
|--------|----------|------|-----------|
| GET | `/api/lists` | Admin | Listar listas |
| POST | `/api/lists` | Admin | Criar lista |
| PUT | `/api/lists/{id}` | Admin | Editar lista |
| DELETE | `/api/lists/{id}` | Admin | Remover lista |
| POST | `/api/lists/{id}/entries` | Admin | Adicionar domínios |
| POST | `/api/lists/reload` | Admin | Recarregar em memória |
| POST | `/api/lists/download` | Admin | Baixar listas externas |
| GET | `/api/lists/{id}/categories` | Admin | Ver categorias da lista |
| PUT | `/api/lists/{id}/categories` | Admin | Definir categorias |

### IP Ranges

| Método | Endpoint | Auth | Descrição |
|--------|----------|------|-----------|
| GET | `/api/ranges` | Admin | Listar ranges |
| POST | `/api/ranges` | Admin | Criar range |
| PUT | `/api/ranges/{id}` | Admin | Editar range |
| DELETE | `/api/ranges/{id}` | Admin | Remover range |

### Cache

| Método | Endpoint | Auth | Descrição |
|--------|----------|------|-----------|
| DELETE | `/api/cache/{domain}` | JWT | Invalidar cache de um domínio |

---

## Configuração — Referência Completa

### Arquivo YAML

| Parâmetro | Default | Descrição |
|-----------|---------|-----------|
| `dns.listen` | `0.0.0.0:53` | Endereço do servidor DNS |
| `dns.upstreams` | Google + Cloudflare | Servidores upstream (fallback automático) |
| `dns.block_ip` | `0.0.0.0` | IP retornado para domínios bloqueados |
| `dns.portal_ip` | `0.0.0.0` | IP do captive portal (use o IP real do servidor) |
| `cache.ttl_floor_seconds` | `30` | TTL mínimo do cache |
| `cache.ttl_ceiling_seconds` | `3600` | TTL máximo do cache |
| `api.listen` | `:8081` | Porta da API REST + Dashboard |
| `api.jwt_secret` | — | **Obrigatório em produção** (32+ caracteres) |
| `api.tls_cert` | — | Caminho do certificado TLS (opcional) |
| `api.tls_key` | — | Caminho da chave TLS (opcional) |
| `captive.listen` | `:80` | Porta do captive portal |
| `captive.session_ttl_hours` | `8` | Duração da sessão do captive portal |
| `database.url` | localhost | URL de conexão PostgreSQL |
| `database.retention_days` | `120` | Dias de retenção dos logs DNS |
| `database.log_buffer_size` | `100000` | Tamanho do buffer do log pipeline |
| `redis.addr` | `localhost:6379` | Endereço do Redis (vazio para desativar) |
| `log.level` | `info` | Nível de log (debug, info, warn, error) |

### Variáveis de ambiente (override do YAML)

| Variável | Override |
|----------|---------|
| `DATABASE_URL` | `database.url` |
| `JWT_SECRET` | `api.jwt_secret` |
| `DNS_LISTEN` | `dns.listen` |

---

## Deploy com Docker

### Docker Compose (produção)

```bash
# Configure o JWT secret
export JWT_SECRET="seu-secret-aleatorio-de-32-caracteres"

# Suba tudo (PostgreSQL + Redis + DNS Filter)
docker-compose up -d

# Verifique os containers
docker ps
```

O `docker-compose.yml` inclui: PostgreSQL (TimescaleDB), Redis e o DNS Filter.

### Build manual da imagem Docker

```bash
docker build -t dnsfilter .
```

---

## Segurança — Recomendações para Produção

1. **JWT Secret:** Use no mínimo 32 caracteres aleatórios. Nunca use o valor padrão.
2. **Senha admin:** Troque a senha padrão (`admin123`) imediatamente após o primeiro login.
3. **Firewall:** Restrinja acesso à porta 8081 (dashboard) apenas para IPs de administradores.
4. **Captive Portal:** A porta 80 deve ser acessível apenas na rede interna.
5. **Banco de dados:** Use senhas fortes e `sslmode=require` na connection string em produção.
6. **HTTPS:** Configure TLS para o dashboard em ambientes que requerem criptografia.
7. **Rate limiting:** Já implementado — 5 req/s no login, 30 req/s na API geral.

---

## Solução de Problemas

### O servidor não inicia na porta 53

A porta 53 requer privilégios elevados. No Windows, abra o terminal como Administrador. No Linux, use `sudo` ou:

```bash
sudo setcap cap_net_bind_service=+ep ./dnsfilter
```

### Domínios bloqueados continuam acessíveis

1. **DNS privado no Android:** Desative em Configurações → DNS privado → Desativado
2. **Cache do browser:** Limpe o cache ou teste em aba anônima
3. **Cache do roteador:** Reinicie o roteador para limpar o cache DNS
4. **Listas não carregadas:** Clique "Baixar Listas Externas" e depois "Recarregar Listas" no dashboard

### Após desligar o filtro, domínios continuam bloqueados

O roteador cacheia respostas DNS. Reinicie o roteador para limpar o cache. Isso pode levar 1-5 minutos.

### Redis não conecta

O Redis é opcional. Se indisponível, o servidor continua funcionando apenas com cache L1 (in-memory). A mensagem "Redis L2: indisponível" é apenas um aviso.

### PostgreSQL não conecta

O servidor DNS continua resolvendo queries mesmo sem PostgreSQL. Listas de arquivo (`blocklist.txt`, `allowlist.txt`) são carregadas como fallback. Logs e dashboard ficam indisponíveis.

---

## Estrutura do Projeto

```
cmd/dnsfilter/       → Entry point do binário
cmd/setup/           → Utilitário para criar usuário admin
internal/api/        → API REST + JWT + dashboard embarcado
internal/cache/      → Cache DNS L1 (in-memory) + L2 (Redis)
internal/captive/    → Captive Portal (login HTTP)
internal/config/     → Configuração YAML + env overrides
internal/dns/        → Servidor DNS (listener, handler, upstream)
internal/filter/     → Motor de filtragem (blacklist, whitelist, categorias)
internal/identity/   → Identificação de clientes (IP/CIDR, sessões)
internal/logger/     → Logging estruturado com níveis
internal/logging/    → Pipeline assíncrono de logs DNS
internal/store/      → PostgreSQL + migrations + CRUD
web/                 → Frontend React (código-fonte)
configs/             → Arquivos de configuração YAML
```

---

## Portas Utilizadas

| Porta | Protocolo | Serviço | Acesso |
|-------|-----------|---------|--------|
| 53 | UDP + TCP | Servidor DNS | Toda a rede |
| 80 | TCP | Captive Portal | Rede interna |
| 8081 | TCP | Dashboard + API | Apenas administradores |
| 5432 | TCP | PostgreSQL | Apenas localhost |
| 6379 | TCP | Redis | Apenas localhost |

---

## Licença

Este projeto está em desenvolvimento ativo. Uso interno permitido.
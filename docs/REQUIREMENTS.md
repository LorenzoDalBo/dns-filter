# DNS Filter Empresarial — Requisitos e Arquitetura

## Visão Geral do Projeto

Filtro DNS empresarial para redes de 500 a 5000 dispositivos, inspirado no NxFilter. Construído do zero em Go com a biblioteca miekg/dns. Deploy on-premise, binário único, interface em Português (BR).

---

## Contexto Técnico

- **Backend:** Go + miekg/dns (servidor DNS construído do zero)
- **Banco de dados:** PostgreSQL + TimescaleDB (logs + configuração)
- **Cache:** L1 in-memory + L2 Redis
- **Frontend:** React + Vite + TanStack Table + Shadcn/ui
- **Deploy:** Binário único (DNS + API REST + Captive Portal), on-premise
- **Referência de implementação:** Blocky (código-fonte como referência, não como base)

---

## Requisitos Funcionais

### RF01 — Servidor DNS Core

- RF01.1: Escutar e responder queries DNS na porta 53 (UDP e TCP).
- RF01.2: Suportar os tipos de registro mais comuns: A, AAAA, CNAME, MX, NS, TXT, SOA, PTR, SRV.
- RF01.3: Encaminhar queries não cacheadas para upstream resolvers configuráveis (ex: 8.8.8.8, 1.1.1.1).
- RF01.4: Suportar múltiplos upstream resolvers com fallback automático (se o primário falhar, usa o secundário).
- RF01.5: Respeitar o TTL dos registros DNS retornados pelo upstream.
- RF01.6: Fazer TCP fallback automático quando a resposta DNS exceder 512 bytes (truncation).
- RF01.7: Responder com NOERROR (answer vazia) para tipos de registro não suportados, nunca SERVFAIL.

### RF02 — Cache DNS

- RF02.1: Manter cache em memória (L1) para os domínios mais consultados.
- RF02.2: Manter cache secundário no Redis (L2) para o restante.
- RF02.3: Respeitar o TTL original do registro DNS, com floor configurável (default: 30s) e ceiling configurável (default: 1h).
- RF02.4: Invalidar entradas de cache por domínio específico via API (para troubleshooting).
- RF02.5: Expor métricas de cache hit/miss via API para o dashboard.

### RF03 — Motor de Filtragem (Policy Engine)

- RF03.1: Avaliar toda query DNS contra as políticas do grupo do cliente, mesmo com cache hit.
- RF03.2: Suportar blacklists (bloquear domínios listados) e whitelists (permitir independente de blacklist).
- RF03.3: Suportar categorias de domínio (ex: malware, adulto, redes sociais, jogos, streaming).
- RF03.4: Cada grupo de usuários pode ter uma política distinta (grupo "Devs" permite tudo, grupo "Visitantes" bloqueia redes sociais e streaming).
- RF03.5: Whitelists têm prioridade sobre blacklists.
- RF03.6: Suportar wildcard blocking (ex: *.ads.example.com bloqueia todos os subdomínios).
- RF03.7: Quando um domínio é bloqueado, retornar um IP configurável (ex: IP do servidor de block page).
- RF03.8: Blacklists e whitelists devem ser carregadas em memória (hashmap) para lookup O(1).
- RF03.9: Recarregar listas em memória sem restart do serviço (hot reload via LISTEN/NOTIFY do PostgreSQL).

### RF04 — Listas de Bloqueio

- RF04.1: Suportar listas manuais (admin cadastra domínios individuais ou em lote via dashboard).
- RF04.2: Suportar listas externas via URL (ex: StevenBlack, EasyList, energized).
- RF04.3: Atualizar listas externas automaticamente em intervalo configurável (default: 24h).
- RF04.4: Suportar formato hosts file (ignorando prefixo 127.0.0.1 / 0.0.0.0) e formato domínio puro (um por linha).
- RF04.5: Permitir associar cada lista a uma ou mais categorias.
- RF04.6: Permitir ativar/desativar uma lista inteira sem deletá-la.
- RF04.7: Exibir no dashboard o total de domínios carregados por lista e a data da última atualização.

### RF05 — Identificação de Clientes

- RF05.1: Identificar clientes por IP individual ou range CIDR.
- RF05.2: Mapear IP/CIDR → Usuário → Grupo → Política.
- RF05.3: Suportar associação estática (admin vincula IP ou range a um grupo diretamente, sem login).
- RF05.4: Suportar autenticação via Captive Portal (login obrigatório para faixas configuradas).
- RF05.5: Sessões autenticadas possuem TTL configurável (default: 8h). Após expiração, o ciclo de login recomeça.
- RF05.6: Cada faixa de IP tem um auth_mode configurável: none (política direto), captive_portal (login obrigatório).
- RF05.7: IPs não reconhecidos e sem faixa configurada aplicam uma política default global (configurável entre bloquear tudo ou permitir tudo).
- RF05.8: O lookup de identidade deve ser in-memory para não impactar a latência do hot path DNS.
- RF05.9: Prever extensibilidade para integração futura com AD/LDAP (Fase 2) sem refatoração da interface do Identity Resolver.

### RF06 — Captive Portal

- RF06.1: Servir página de login HTTP na porta 80 do servidor.
- RF06.2: Quando um IP desconhecido (com auth_mode: captive_portal) faz uma query DNS, responder com o IP do próprio servidor, redirecionando o browser para a página de login.
- RF06.3: Após autenticação bem-sucedida, registrar sessão (IP → Usuário → Grupo) no Identity Cache.
- RF06.4: A página de login deve ser funcional e apresentável, em Português (BR).
- RF06.5: Exibir mensagem de erro clara para credenciais inválidas.
- RF06.6: Após login, redirecionar o usuário para a URL que ele originalmente tentou acessar (quando possível).

### RF07 — Logging e Auditoria

- RF07.1: Registrar toda query DNS processada com: timestamp, IP do cliente, usuário (se identificado), grupo, domínio consultado, tipo de query, ação tomada (permitido/bloqueado/cache), razão do bloqueio (se aplicável), categoria, IP da resposta, latência, upstream utilizado.
- RF07.2: O pipeline de log deve ser assíncrono (fire-and-forget via channel bufferizado).
- RF07.3: Gravar logs em batch no PostgreSQL + TimescaleDB (acumular por 1s ou 5000 registros, o que vier primeiro).
- RF07.4: Se o buffer encher (ex: PostgreSQL offline), descartar logs silenciosamente e registrar warning. Nunca bloquear a resolução DNS por causa de log.
- RF07.5: Suportar retenção configurável (default: 120 dias), com remoção automática de partições antigas.
- RF07.6: Logs particionados por dia (TimescaleDB hypertable).

### RF08 — Dashboard

- RF08.1: Tela de visão geral com estatísticas: total de queries (hoje/semana/mês), percentual bloqueado, top domínios consultados, top domínios bloqueados, top clientes por volume.
- RF08.2: Tela de logs filtráveis com os seguintes filtros, combináveis simultaneamente:
  - Data e horário (range).
  - Usuário ou IP de origem.
  - Domínio consultado (busca parcial e exata).
  - Ação tomada (permitido / bloqueado / cache).
  - Categoria.
- RF08.3: Paginação server-side para a tabela de logs (nunca carregar todos os registros no frontend).
- RF08.4: Tela de gerenciamento de usuários (CRUD).
- RF08.5: Tela de gerenciamento de grupos e políticas (CRUD + associação de categorias bloqueadas por grupo).
- RF08.6: Tela de gerenciamento de listas de bloqueio (adicionar listas manuais e externas, ativar/desativar, forçar atualização).
- RF08.7: Tela de gerenciamento de ranges de IP (associar faixa a grupo, definir auth_mode).
- RF08.8: Toda a interface em Português (BR).

### RF09 — Controle de Acesso ao Dashboard

- RF09.1: Autenticação por login e senha para acessar o dashboard.
- RF09.2: Suportar no mínimo 2 perfis de permissão: Administrador (acesso total) e Visualizador (apenas consulta de logs e estatísticas, sem alterar configurações).
- RF09.3: Administradores podem criar, editar e desativar outros usuários do dashboard.
- RF09.4: Sessões do dashboard com expiração configurável.
- RF09.5: Senhas armazenadas com hash seguro (bcrypt ou argon2).

### RF10 — API REST

- RF10.1: Expor endpoints REST para todas as operações do dashboard (CRUD de usuários, grupos, políticas, listas, ranges, consulta de logs).
- RF10.2: Autenticação via JWT com expiração configurável.
- RF10.3: Endpoints protegidos por perfil de permissão (Administrador vs Visualizador).
- RF10.4: Endpoint para forçar reload de listas e políticas em memória.
- RF10.5: Endpoint para invalidar cache DNS de um domínio específico.
- RF10.6: Endpoint para métricas operacionais (cache hit ratio, queries/segundo, uptime, total de domínios bloqueados carregados).

---

## Requisitos Não Funcionais

### RNF01 — Performance

- RNF01.1: Latência do hot path DNS ≤ 5ms para cache hit (incluindo Identity Resolver + Policy Engine + cache lookup).
- RNF01.2: Suportar no mínimo 5.000 queries DNS por segundo com latência estável (rede de 5000 dispositivos).
- RNF01.3: Identity Resolver, Policy Engine e blacklists operam exclusivamente in-memory — sem I/O de disco ou rede no hot path DNS (exceto cache miss no Redis L2 e upstream).
- RNF01.4: O pipeline de log assíncrono nunca deve adicionar latência ao hot path DNS.
- RNF01.5: O dashboard deve carregar páginas de log com até 100 registros por página em ≤ 2 segundos, mesmo com 100M+ de registros na base.

### RNF02 — Disponibilidade e Resiliência

- RNF02.1: O servidor DNS deve continuar resolvendo queries mesmo se o PostgreSQL estiver offline (listas já estão em memória; logs são descartados temporariamente).
- RNF02.2: O servidor DNS deve continuar resolvendo queries mesmo se o Redis estiver offline (fallback para cache L1 in-memory apenas).
- RNF02.3: Se todos os upstream resolvers falharem, retornar SERVFAIL ao cliente (comportamento padrão DNS).
- RNF02.4: Graceful shutdown: ao receber SIGTERM, terminar queries em andamento antes de encerrar.

### RNF03 — Segurança

- RNF03.1: A API REST e o dashboard devem ser acessíveis apenas via HTTPS (TLS configurável).
- RNF03.2: Senhas de usuários do dashboard armazenadas com hash seguro (bcrypt ou argon2).
- RNF03.3: Tokens JWT com expiração e refresh token.
- RNF03.4: Rate limiting na API REST e no captive portal (prevenir brute force).
- RNF03.5: O captive portal HTTP (porta 80) deve ser acessível apenas na rede interna (orientação de firewall na documentação).

### RNF04 — Manutenibilidade

- RNF04.1: Configuração centralizada via arquivo YAML para parâmetros de inicialização (portas, endereço do PostgreSQL, Redis, upstream DNS, etc).
- RNF04.2: Configuração dinâmica (usuários, grupos, políticas, listas) gerenciada via banco de dados + API, nunca por edição manual de arquivo.
- RNF04.3: Hot reload de políticas e listas sem restart (via LISTEN/NOTIFY do PostgreSQL).
- RNF04.4: Logs estruturados do próprio serviço (não confundir com logs DNS) em formato legível com níveis (debug, info, warn, error).
- RNF04.5: Migrations versionadas para o schema do banco de dados (golang-migrate ou similar).

### RNF05 — Escalabilidade

- RNF05.1: A arquitetura deve permitir escalar horizontalmente no futuro (múltiplas instâncias do servidor DNS atrás de um load balancer), usando Redis como cache compartilhado.
- RNF05.2: O banco de dados de logs (TimescaleDB) deve suportar compressão e retenção automática para viabilizar 120+ dias de histórico sem crescimento descontrolado de disco.
- RNF05.3: O carregamento de blacklists em memória deve suportar até 2 milhões de domínios com consumo de memória ≤ 500MB.

### RNF06 — Observabilidade

- RNF06.1: Expor métricas operacionais via API (queries/segundo, cache hit ratio, latência média, uptime, total de domínios bloqueados).
- RNF06.2: O serviço deve logar eventos relevantes (inicialização, reload de listas, falha de upstream, perda de logs por buffer cheio).
- RNF06.3: Health check endpoint (HTTP) para monitoramento externo.

### RNF07 — Deploy e Operação

- RNF07.1: Distribuído como binário único (Go, compilação estática).
- RNF07.2: Suportar deploy via Docker (Dockerfile + docker-compose com PostgreSQL + Redis inclusos).
- RNF07.3: Documentação de instalação e configuração inicial (em Português).
- RNF07.4: Primeira execução deve criar as tabelas no banco automaticamente (auto-migrate).

---

## Fora do Escopo (v1)

- Cotas de tempo por usuário/grupo.
- Billing ou licenciamento.
- DNS over TLS (DoT) e DNS over HTTPS (DoH) — previsto para Fase 2.
- Integração com Active Directory / LDAP — previsto para Fase 2.
- Suporte a múltiplos idiomas (i18n) — v1 é apenas Português (BR).
- Aplicativo mobile ou agente instalado nos endpoints.
- DNSSEC validation (queries são repassadas ao upstream que valida).

---

## Fases do Projeto

### Fase 1 (MVP)
Todos os requisitos RF01 a RF10 e RNF01 a RNF07 listados acima.

### Fase 2 (Evolução)
- DoT / DoH (RF adicional).
- Integração AD/LDAP via Event 4624 (extensão do RF05).
- Block page customizável (RF adicional vinculado ao RF03.7).
- Internacionalização (i18n) para inglês.

---

## Decisões Técnicas Consolidadas

1. **Servidor DNS:** Go + miekg/dns, construído do zero (Opção C). Blocky como referência.
2. **Cache:** L1 in-memory (top domínios) + L2 Redis. Redis opcional na v1 (pode iniciar só com L1).
3. **Banco de logs:** PostgreSQL + TimescaleDB, particionamento diário, compressão nativa.
4. **Schema de logs:** Sem UUID como PK. SMALLINT para enums. Índice trigram para busca parcial de domínio.
5. **Blacklists:** In-memory (hashmap). PostgreSQL como fonte de verdade. Hot reload via LISTEN/NOTIFY.
6. **Identity:** In-memory (sessions + IP ranges). 3 modos: IP estático, CIDR, Captive Portal.
7. **Log pipeline:** Assíncrono (channel buffer 100k). Batch insert. Descarte silencioso se buffer cheio.
8. **API ↔ DNS Engine:** Mesmo binário. LISTEN/NOTIFY para reload de configuração.
9. **Permissões dashboard:** 2 perfis iniciais (Administrador, Visualizador).
10. **Deploy:** Binário único + Docker Compose (PostgreSQL + Redis).

---

## Roadmap de Implementação

- **Fase 0 (semana 1-2):** Aprendizado de Go (Tour of Go + exercícios práticos).
- **Fase 1 (semana 3):** DNS Echo Server — menor servidor funcional.
- **Fase 2 (semana 4):** DNS Forwarder — encaminhamento para upstream.
- **Fase 3 (semana 5-6):** Cache DNS in-memory com TTL.
- **Fase 4 (semana 7-8):** Filtragem básica com blacklist de arquivo.
- **Fase 5 (semana 9-10):** PostgreSQL + modelo de dados completo.
- **Fase 6 (semana 11-13):** Identity Resolver + Captive Portal.
- **Fase 7 (semana 14-16):** API REST + Async Log Pipeline.
- **Fase 8 (semana 17-20):** Dashboard React com logs filtráveis.

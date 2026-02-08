# Phase 4 — ChatGPT OAuth Gateway (Authorization Code + PKCE)

## Projeto Memory Cloud
- **Local:** /Users/wagner/Projetos/memory-cloud
- **Repo:** https://github.com/wagneradl/mc-v1.git
- **Branch:** main
- **VPS:** api.wagnerlima.cc (46.225.69.233), SSH: `ssh deploy@46.225.69.233`
- **Deploy path:** /opt/mcp-hub/

## Leia antes de começar
```
docs/ARCHITECTURE.md
docs/INFRASTRUCTURE.md
docs/MEMORY-MCP-SPEC.md
docs/CLIENT-CONFIG.md
Caddyfile
docker-compose.yml
.env.example
```

---

## CONTEXTO CRÍTICO: Resultado da pesquisa sobre OAuth do ChatGPT

A pesquisa (Perplexity Pro, fev/2026) revelou que o ChatGPT MCP Apps **NÃO** usa Client Credentials. O fluxo real é:

### O que o ChatGPT faz (OAuth 2.1 — Authorization Code + PKCE + DCR)

```
1. ChatGPT tenta GET /servers/memory/sse → recebe 401
2. ChatGPT GET /.well-known/oauth-protected-resource → descobre o IdP
3. ChatGPT GET {idp}/.well-known/oauth-authorization-server → descobre endpoints
4. ChatGPT POST {registration_endpoint} → Dynamic Client Registration → recebe client_id
5. ChatGPT redireciona o usuário para {authorization_endpoint}?response_type=code&code_challenge=...
6. Usuário faz "login" (insere senha/PIN) → servidor redireciona para redirect_uri com ?code=...
7. ChatGPT POST {token_endpoint} com grant_type=authorization_code, code, code_verifier (PKCE)
8. Servidor retorna { access_token, token_type: "Bearer", expires_in, refresh_token? }
9. ChatGPT usa Authorization: Bearer {access_token} em TODAS as requests MCP subsequentes
10. Quando expira: usa refresh_token ou re-roda o flow completo
```

### Endpoints obrigatórios (descobertos via .well-known)

| Endpoint | Método | Propósito |
|----------|--------|-----------|
| `/.well-known/oauth-protected-resource` | GET | No domínio MCP. Diz ao ChatGPT onde está o IdP |
| `/.well-known/oauth-authorization-server` | GET | No domínio do IdP. Metadata RFC 8414 |
| `/oauth/register` | POST | Dynamic Client Registration (DCR) |
| `/oauth/authorize` | GET/POST | Tela de login/consent → redirect com auth code |
| `/oauth/token` | POST | Troca auth code por access_token |

### Formato do token request (form-urlencoded POST)
```
grant_type=authorization_code
code=<auth_code>
redirect_uri=<redirect URI do DCR>
client_id=<client_id do DCR>
code_verifier=<PKCE verifier>
resource=https://api.wagnerlima.cc
```

ChatGPT **NÃO envia client_secret** no token request — usa PKCE (public client).

### Formato da token response
```json
{
  "access_token": "{MCP_BEARER_TOKEN}",
  "token_type": "Bearer",
  "expires_in": 86400,
  "refresh_token": "optional"
}
```

### Requisitos .well-known (CRÍTICO)
- ChatGPT é **estrito** com os paths `.well-known` — devem estar na **raiz do domínio**
- ✅ `https://api.wagnerlima.cc/.well-known/oauth-protected-resource`
- ❌ `https://api.wagnerlima.cc/oauth/.well-known/...`

### SSE + OAuth
- Nenhum requisito especial além do Bearer header padrão
- Token enviado no início da conexão SSE, não renova mid-stream
- Caddy deve passar `Authorization` header e não bufferizar SSE (já configurado)

---

## Estado atual
Phases 1-3 completas. VPS rodando com 8 MCPs, auth via Bearer token no header Authorization, TLS via Cloudflare Origin Certificate. Clientes configurados: Claude Desktop (stdio bridge) e Claude Code (HTTP direto). Tudo funcionando.

## Problema
O ChatGPT MCP Apps não envia headers customizados. Suporta apenas OAuth 2.1 (Authorization Code + PKCE + DCR) para obter o Bearer token automaticamente.

## Solução: Mini OAuth Authorization Server

Um micro-serviço Go que implementa o mínimo necessário do OAuth 2.1 para o ChatGPT. Após o flow, o access_token retornado é o próprio MCP_BEARER_TOKEN existente — **zero mudança no Caddy auth ou no mcp-proxy**.

### Diagrama de fluxo

```
ChatGPT                       Caddy                   oauth-server           mcp-proxy
  │                             │                          │                     │
  ├─ GET /servers/X/sse ───────►│─ sem Bearer ─► 401       │                     │
  │◄── 401 ────────────────────│                           │                     │
  │                             │                           │                     │
  ├─ GET /.well-known/          │                           │                     │
  │  oauth-protected-resource ─►│── (sem auth) ───────────►│                     │
  │◄── {resource, auth_servers} │◄─────────────────────────│                     │
  │                             │                           │                     │
  ├─ GET /.well-known/          │                           │                     │
  │  oauth-authorization-server►│── (sem auth) ───────────►│                     │
  │◄── {endpoints metadata} ───│◄─────────────────────────│                     │
  │                             │                           │                     │
  ├─ POST /oauth/register ─────►│── (sem auth) ───────────►│  DCR               │
  │◄── {client_id} ───────────│◄─────────────────────────│                     │
  │                             │                           │                     │
  ├─ Redirect → /oauth/authorize│                           │                     │
  │  ?response_type=code        │                           │                     │
  │  &client_id=...             │                           │                     │
  │  &code_challenge=...  ─────►│── (sem auth) ───────────►│  Login page        │
  │◄── HTML form ──────────────│◄─────────────────────────│                     │
  │                             │                           │                     │
  ├─ POST /oauth/authorize ────►│── (sem auth) ───────────►│  Validate password │
  │◄── 302 redirect_uri?code= ─│◄─────────────────────────│  + redirect        │
  │                             │                           │                     │
  ├─ POST /oauth/token ────────►│── (sem auth) ───────────►│  Code → Bearer     │
  │◄── {access_token: BEARER} ─│◄─────────────────────────│                     │
  │                             │                           │                     │
  ├─ GET /servers/X/sse ───────►│─ Bearer ✓ ──────────────┼────────────────────►│
  │◄── SSE stream ─────────────│◄─────────────────────────┼─────────────────────│
```

---

## Tasks

### Task 4.1 — OAuth Server: Estrutura e metadata endpoints

Crie `oauth-server/` com:

**`main.go`** — Servidor HTTP com os handlers abaixo.

**Variáveis de ambiente:**
- `OAUTH_AUTHORIZE_PASSWORD` — senha/PIN que o usuário insere na tela de login (gerar com `openssl rand -hex 16`)
- `MCP_BEARER_TOKEN` — já existente, é o access_token retornado
- `OAUTH_SERVER_BASE_URL` — URL pública do OAuth server (ex: `https://api.wagnerlima.cc`)
- `MCP_RESOURCE_URL` — URL canônica do recurso MCP (ex: `https://api.wagnerlima.cc`)
- `PORT` — porta interna (default: 8090)

**Endpoint 1:** `GET /.well-known/oauth-protected-resource`
```json
{
  "resource": "https://api.wagnerlima.cc",
  "authorization_servers": ["https://api.wagnerlima.cc"]
}
```
Nota: IdP e MCP no mesmo domínio (simplifica). O Caddy roteia `/.well-known/*` e `/oauth/*` pro oauth-server, e o resto pro mcp-proxy.

**Endpoint 2:** `GET /.well-known/oauth-authorization-server`
RFC 8414 metadata:
```json
{
  "issuer": "https://api.wagnerlima.cc",
  "authorization_endpoint": "https://api.wagnerlima.cc/oauth/authorize",
  "token_endpoint": "https://api.wagnerlima.cc/oauth/token",
  "registration_endpoint": "https://api.wagnerlima.cc/oauth/register",
  "response_types_supported": ["code"],
  "grant_types_supported": ["authorization_code", "refresh_token"],
  "token_endpoint_auth_methods_supported": ["none"],
  "code_challenge_methods_supported": ["S256"]
}
```

**Requisitos:**
- Zero dependências externas (stdlib Go)
- Dockerfile multi-stage (golang:1.24-alpine → alpine)
- GET /health → 200 OK

### Task 4.2 — OAuth Server: Dynamic Client Registration (DCR)

**Endpoint:** `POST /oauth/register`

Aceita JSON:
```json
{
  "client_name": "ChatGPT",
  "redirect_uris": ["https://chatgpt.com/aip/g-..."],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "none"
}
```

Retorna:
```json
{
  "client_id": "<uuid gerado>",
  "client_name": "ChatGPT",
  "redirect_uris": ["https://chatgpt.com/aip/g-..."],
  "grant_types": ["authorization_code", "refresh_token"],
  "response_types": ["code"],
  "token_endpoint_auth_method": "none"
}
```

**Storage:** In-memory (map[string]*Client, protegido por sync.RWMutex). Clientes registrados sobrevivem enquanto o container estiver rodando — aceitável para single-user.

### Task 4.3 — OAuth Server: Authorization Endpoint

**Endpoint:** `GET /oauth/authorize`

Query params recebidos do ChatGPT:
- `response_type=code`
- `client_id=<do DCR>`
- `redirect_uri=<do DCR>`
- `state=<opaco, retornar igual>`
- `code_challenge=<S256 hash>`
- `code_challenge_method=S256`
- `resource=https://api.wagnerlima.cc`

**Comportamento GET:**
1. Validar: client_id existe (via DCR), redirect_uri bate com o registrado
2. Renderizar página HTML de login com:
   - Branding minimalista ("Memory Cloud — Authorize Access")
   - Campo de senha/PIN
   - Botão "Authorize"
   - Hidden fields: todos os query params
3. Template HTML embutido no binário (embed.FS ou string literal)

**Endpoint:** `POST /oauth/authorize`

Form fields: mesmos query params + `password`

**Comportamento POST:**
1. Validar password contra `OAUTH_AUTHORIZE_PASSWORD`
2. Se inválido: re-renderizar form com erro "Invalid password"
3. Se válido:
   - Gerar auth code (uuid ou random hex, 64 chars)
   - Armazenar: code → {client_id, redirect_uri, code_challenge, resource, created_at}
   - Auth codes expiram em 5 minutos
   - Redirect 302 → `{redirect_uri}?code={auth_code}&state={state}`

**Storage:** In-memory (map[string]*AuthCode, com cleanup periódico de expirados).

### Task 4.4 — OAuth Server: Token Endpoint

**Endpoint:** `POST /oauth/token`

Content-Type: `application/x-www-form-urlencoded`

**Grant type: authorization_code**

Campos:
- `grant_type=authorization_code`
- `code=<auth code>`
- `redirect_uri=<deve bater com o do code>`
- `client_id=<deve bater com o do code>`
- `code_verifier=<PKCE verifier>`
- `resource=<deve bater>`

Validação PKCE:
```
expected = BASE64URL(SHA256(code_verifier))
expected == code_challenge armazenado
```

Se válido, retornar:
```json
{
  "access_token": "{MCP_BEARER_TOKEN}",
  "token_type": "Bearer",
  "expires_in": 86400,
  "refresh_token": "<random hex 64>"
}
```

Invalidar o auth code após uso (one-time use).

**Grant type: refresh_token**

Campos:
- `grant_type=refresh_token`
- `refresh_token=<do token response anterior>`
- `client_id=<do DCR>`

Se válido, retornar novo access_token (mesmo MCP_BEARER_TOKEN) + novo refresh_token.

**Storage:** In-memory para refresh tokens (map[string]*RefreshToken).

**Erros (JSON):**
- 400: `{"error": "invalid_request", "error_description": "..."}`
- 400: `{"error": "invalid_grant", "error_description": "..."}`
- 400: `{"error": "unsupported_grant_type"}`

### Task 4.5 — Infraestrutura

**docker-compose.yml** — Adicionar serviço:
```yaml
oauth-server:
  build: ./oauth-server
  container_name: mcp-oauth
  restart: unless-stopped
  environment:
    - OAUTH_AUTHORIZE_PASSWORD=${OAUTH_AUTHORIZE_PASSWORD}
    - MCP_BEARER_TOKEN=${MCP_BEARER_TOKEN}
    - OAUTH_SERVER_BASE_URL=https://${DOMAIN}
    - MCP_RESOURCE_URL=https://${DOMAIN}
    - PORT=8090
  expose:
    - "8090"
  networks:
    - mcp-net
  healthcheck:
    test: ["CMD", "wget", "--spider", "-q", "http://localhost:8090/health"]
    interval: 30s
    timeout: 5s
    retries: 3
```

**Caddyfile** — Reestruturar rotas (ORDEM IMPORTA):

```
{$DOMAIN} {
    tls /etc/caddy/certs/origin.pem /etc/caddy/certs/origin-key.pem

    # 1. OAuth discovery + endpoints — SEM auth (são o mecanismo de auth)
    handle /.well-known/oauth-protected-resource {
        reverse_proxy oauth-server:8090
    }

    handle /.well-known/oauth-authorization-server {
        reverse_proxy oauth-server:8090
    }

    handle /oauth/* {
        reverse_proxy oauth-server:8090
    }

    # 2. MCP endpoints — Bearer auth obrigatória
    @authorized {
        header Authorization "Bearer {$MCP_BEARER_TOKEN}"
    }
    handle @authorized {
        reverse_proxy mcp-proxy:8080 {
            flush_interval -1
            transport http {
                read_timeout 0
                write_timeout 0
            }
        }
    }

    # 3. Catch-all — 401
    handle {
        respond "Unauthorized" 401
    }

    log {
        output file /var/log/caddy/access.log
        format json
    }
}
```

**.env.example** — Adicionar:
```bash
# OAuth Authorization Server (Phase 4 — ChatGPT integration)
OAUTH_AUTHORIZE_PASSWORD=   # Password for OAuth login screen (openssl rand -hex 16)
# MCP_BEARER_TOKEN already exists above — reused as access_token
```

### Task 4.6 — Testes locais

1. `docker compose up -d --build`

2. Testar discovery:
```bash
# Protected resource metadata
curl -s https://localhost/.well-known/oauth-protected-resource | jq .

# Authorization server metadata
curl -s https://localhost/.well-known/oauth-authorization-server | jq .
```

3. Testar DCR:
```bash
CLIENT=$(curl -s -X POST https://localhost/oauth/register \
  -H "Content-Type: application/json" \
  -d '{"client_name":"test","redirect_uris":["http://localhost:9999/callback"],"grant_types":["authorization_code"],"response_types":["code"],"token_endpoint_auth_method":"none"}')
CLIENT_ID=$(echo $CLIENT | jq -r .client_id)
echo "Client ID: $CLIENT_ID"
```

4. Testar authorization (manual — abrir no browser):
```
https://localhost/oauth/authorize?response_type=code&client_id={CLIENT_ID}&redirect_uri=http://localhost:9999/callback&state=test123&code_challenge=E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM&code_challenge_method=S256&resource=https://localhost
```
(code_challenge acima é SHA256 de "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")

5. Testar token exchange:
```bash
curl -s -X POST https://localhost/oauth/token \
  -d "grant_type=authorization_code&code={CODE}&redirect_uri=http://localhost:9999/callback&client_id={CLIENT_ID}&code_verifier=dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk&resource=https://localhost" | jq .
```

6. Testar que o Bearer token funciona no MCP:
```bash
TOKEN=$(...)  # access_token do passo anterior
curl -H "Authorization: Bearer $TOKEN" https://localhost/servers/memory/mcp
```

7. **Regression test:** Verificar que Claude Desktop e Claude Code continuam funcionando com Bearer direto.

### Task 4.7 — Deploy VPS

1. SSH: `ssh deploy@46.225.69.233`
2. Gerar senha de produção:
```bash
OAUTH_AUTHORIZE_PASSWORD=$(openssl rand -hex 16)
echo "Senha OAuth: $OAUTH_AUTHORIZE_PASSWORD"  # Guardar!
```
3. Adicionar ao `.env` da VPS
4. `cd /opt/mcp-hub && ./update.sh --rebuild`
5. Testar discovery em produção:
```bash
curl -s https://api.wagnerlima.cc/.well-known/oauth-protected-resource | jq .
curl -s https://api.wagnerlima.cc/.well-known/oauth-authorization-server | jq .
```
6. Regression test: Claude Desktop + Claude Code

### Task 4.8 — Configurar ChatGPT MCP Apps

Após deploy, configurar cada MCP como App no ChatGPT:

| MCP | Name | MCP Server URL | Auth |
|-----|------|---------------|------|
| Memory | Memory Cloud | https://api.wagnerlima.cc/servers/memory/sse | OAuth |
| GitHub | GitHub Remote | https://api.wagnerlima.cc/servers/github/sse | OAuth |
| Brave | Brave Search | https://api.wagnerlima.cc/servers/brave/sse | OAuth |
| Todoist | Todoist | https://api.wagnerlima.cc/servers/todoist/sse | OAuth |
| Git | Git Remote | https://api.wagnerlima.cc/servers/git/sse | OAuth |
| Firecrawl | Firecrawl | https://api.wagnerlima.cc/servers/firecrawl/sse | OAuth |
| Puppeteer | Puppeteer Remote | https://api.wagnerlima.cc/servers/puppeteer/sse | OAuth |
| Thinking | Sequential Thinking | https://api.wagnerlima.cc/servers/thinking/sse | OAuth |

**Não é necessário informar URLs de auth/token manualmente** — o ChatGPT descobre tudo via `.well-known`. Basta selecionar "OAuth" e clicar "Connect". O ChatGPT vai:
1. Descobrir o IdP via `/.well-known/oauth-protected-resource`
2. Se registrar via DCR
3. Redirecionar para a tela de login
4. Você insere a `OAUTH_AUTHORIZE_PASSWORD`
5. Pronto — todas as 8 Apps usam o mesmo token

### Task 4.9 — Documentação

1. Atualizar `docs/CLIENT-CONFIG.md`:
   - Seção ChatGPT com o fluxo OAuth
   - Nota: a senha de login é `OAUTH_AUTHORIZE_PASSWORD` do .env
   - Nota: todos os Apps compartilham o mesmo token pós-auth
2. Atualizar `docs/ARCHITECTURE.md` com o componente oauth-server
3. Atualizar `docs/INFRASTRUCTURE.md` com as novas env vars
4. Atualizar `DEVELOPMENT-PLAN.md` com Phase 4 completa
5. Commit: `feat: add OAuth 2.1 Authorization Server for ChatGPT MCP Apps`
6. Push

---

## Restrições de segurança

- Os paths `/.well-known/*` e `/oauth/*` são os ÚNICOS sem Bearer auth no Caddy
- `OAUTH_AUTHORIZE_PASSWORD` é a barreira de acesso — alta entropia (128 bits / hex 16)
- Auth codes são single-use e expiram em 5 minutos
- PKCE (S256) previne interceptação de auth codes
- O `access_token` retornado é o MCP_BEARER_TOKEN existente — Caddy valida normalmente
- DCR é aberto (qualquer um pode registrar um client_id), mas sem a senha não obtém token
- Rate limiting: considerar no Caddy para `/oauth/authorize` POST (prevenir brute force de senha)
- Refresh tokens em memória — reiniciar container invalida tokens (aceitável, ChatGPT re-autentica)

## Complexidade estimada

| Componente | Linhas estimadas |
|------------|-----------------|
| main.go (handlers + server) | ~300-400 |
| templates (login HTML) | ~50-80 |
| Dockerfile | ~15 |
| Caddyfile changes | ~15 |
| docker-compose changes | ~15 |
| **Total novo código** | **~400-500** |

É mais complexo que o Client Credentials originalmente planejado, mas é a forma correta. O ChatGPT exige esse fluxo — não há atalho.

## Entregáveis

- [ ] oauth-server/main.go (metadata + DCR + authorize + token)
- [ ] oauth-server/Dockerfile
- [ ] Página HTML de login (embutida no binário)
- [ ] docker-compose.yml atualizado
- [ ] Caddyfile atualizado
- [ ] .env.example atualizado
- [ ] Testes locais passando (discovery, DCR, auth flow, token, MCP access)
- [ ] Deploy VPS funcionando
- [ ] ChatGPT Apps configurados e testados
- [ ] Docs atualizados
- [ ] Commit + push

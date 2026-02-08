# iOS Shortcuts & Scriptable — Memory Cloud Mobile Toolkit

Automações para consumir o Memory Cloud MCP Hub diretamente do iPhone/iPad.

---

## Pré-requisitos

- **iOS Shortcuts** (nativo)
- **Scriptable** (App Store, gratuito) — executa JavaScript, ideal para JSON-RPC
- **a-Shell mini** (opcional) — terminal para debug e curl direto
- **Bearer Token** salvo no Shortcuts como variável reutilizável

---

## Configuração Inicial

### 1. Criar Shortcut "MCP Config" (variáveis globais)

Este shortcut não faz nada sozinho — serve como referência de variáveis.
Salve estes valores num **arquivo JSON no iCloud Drive** (`Shortcuts/mcp-config.json`):

```json
{
  "baseUrl": "https://api.wagnerlima.cc",
  "bearerToken": "SEU_MCP_BEARER_TOKEN",
  "defaultProject": "memory-cloud"
}
```

> ⚠️ O token é sensível. O arquivo fica no iCloud do seu Apple ID, não é compartilhado.

### 2. Instalar o script base no Scriptable

Crie um novo script no Scriptable chamado **"MCP Call"**:

```javascript
// MCP Call — Base script for Memory Cloud JSON-RPC
// Called by iOS Shortcuts via Scriptable URL scheme

const CONFIG_PATH = "mcp-config.json"

async function loadConfig() {
  const fm = FileManager.iCloud()
  const dir = fm.documentsDirectory()
  const path = fm.joinPath(dir, CONFIG_PATH)
  
  // If config is in Shortcuts folder
  const shortcutsPath = fm.joinPath(
    fm.libraryDirectory().replace("/Scriptable", "/Shortcuts"),
    CONFIG_PATH
  )
  
  let configPath = fm.fileExists(path) ? path : shortcutsPath
  if (!fm.fileExists(configPath)) {
    throw new Error("Config not found. Create mcp-config.json in iCloud/Scriptable/")
  }
  await fm.downloadFileFromiCloud(configPath)
  return JSON.parse(fm.readString(configPath))
}

async function mcpCall(server, method, params) {
  const config = await loadConfig()
  const url = `${config.baseUrl}/servers/${server}/mcp`
  const headers = {
    "Content-Type": "application/json",
    "Authorization": `Bearer ${config.bearerToken}`
  }

  // Step 1: Initialize
  const initReq = new Request(url)
  initReq.method = "POST"
  initReq.headers = headers
  initReq.body = JSON.stringify({
    jsonrpc: "2.0",
    id: 1,
    method: "initialize",
    params: {
      protocolVersion: "2025-03-26",
      capabilities: {},
      clientInfo: { name: "ios-scriptable", version: "1.0" }
    }
  })
  const initResp = await initReq.loadJSON()

  // Step 2: Capture session (if server returns it)
  // mcp-proxy may use Mcp-Session-Id header
  const sessionHeaders = { ...headers }
  const sessionId = initReq.response?.headers?.["Mcp-Session-Id"]
    || initReq.response?.headers?.["mcp-session-id"]
  if (sessionId) {
    sessionHeaders["Mcp-Session-Id"] = sessionId
  }

  // Step 3: Send initialized notification
  const notifReq = new Request(url)
  notifReq.method = "POST"
  notifReq.headers = sessionHeaders
  notifReq.body = JSON.stringify({
    jsonrpc: "2.0",
    method: "notifications/initialized"
  })
  await notifReq.loadString()

  // Step 4: Call the actual tool
  const toolReq = new Request(url)
  toolReq.method = "POST"
  toolReq.headers = sessionHeaders
  toolReq.body = JSON.stringify({
    jsonrpc: "2.0",
    id: 2,
    method: method,
    params: params
  })
  return await toolReq.loadJSON()
}

// Entry point — receives args from Shortcuts
async function main() {
  const args = args?.shortcutParameter
  if (!args) {
    // Direct run — test mode
    const result = await mcpCall("memory", "tools/call", {
      name: "get_current_project",
      arguments: {}
    })
    QuickLook.present(result)
    return
  }

  const { server, tool, arguments: toolArgs } = JSON.parse(args)
  const result = await mcpCall(server || "memory", "tools/call", {
    name: tool,
    arguments: toolArgs || {}
  })

  // Return to Shortcuts
  Script.setShortcutOutput(JSON.stringify(result, null, 2))
  Script.complete()
}

await main()
```

> Este é o script-base. Todos os Shortcuts abaixo chamam ele passando parâmetros diferentes.

---

## Padrão dos Shortcuts

Todos os Shortcuts seguem o mesmo padrão:

```
1. [Input do usuário] (se necessário)
2. Montar JSON com server + tool + arguments
3. Executar Scriptable "MCP Call" passando o JSON
4. Exibir resultado (Quick Look ou notificação)
```

No iOS Shortcuts, a ação é:
- **"Run Scriptable Script"** → selecionar "MCP Call"
- **Parameter:** o JSON montado no passo anterior
- **Wait for Result:** ON

---

## Shortcuts — Memory Cloud (Integridade)

### MC01: Verificar Projeto Ativo

**Propósito:** Confirma qual projeto está ativo antes de qualquer operação.

**Shortcut:**
```
1. Run Scriptable "MCP Call" with parameter:
   {"server":"memory","tool":"get_current_project","arguments":{}}
2. Show Result
```

---

### MC02: Trocar Projeto

**Propósito:** Alterna entre projetos.

**Shortcut:**
```
1. Run Scriptable "MCP Call" with parameter:
   {"server":"memory","tool":"list_projects","arguments":{"status":"active"}}
2. Parse JSON → extract project names into List
3. Choose from List → save as "projectName"
4. Run Scriptable "MCP Call" with parameter:
   {"server":"memory","tool":"switch_project","arguments":{"name":"[projectName]"}}
5. Show Notification: "Projeto ativo: [projectName]"
```

---

### MC03: Buscar Memória

**Propósito:** Busca rápida no knowledge graph. O shortcut mais usado.

**Shortcut:**
```
1. Ask for Input (Text): "O que buscar?"  → save as "query"
2. Run Scriptable "MCP Call" with parameter:
   {"server":"memory","tool":"search_nodes","arguments":{"query":"[query]"}}
3. Show Result (Quick Look)
```

---

### MC04: Salvar Ideia

**Propósito:** Captura rápida de ideias no projeto ativo.

**Shortcut:**
```
1. Ask for Input (Text): "Título da ideia?" → save as "title"
2. Ask for Input (Text): "Descrição?" → save as "description"
3. Get Current Date → format "yyyy-MM-dd" → save as "today"
4. Run Scriptable "MCP Call" with parameter:
   {
     "server": "memory",
     "tool": "search_nodes",
     "arguments": {"query": "[title]"}
   }
5. If result contains entities → Show Alert "Já existe! Verifique." → Stop
6. Run Scriptable "MCP Call" with parameter:
   {
     "server": "memory",
     "tool": "create_entities",
     "arguments": {
       "entities": [{
         "name": "[title]",
         "entity_type": "concept",
         "observations": ["[description]", "Data: [today]", "Status: ideia"]
       }]
     }
   }
7. Show Notification: "Ideia salva: [title] ✅"
```

> Note o passo 5: **search before create** — protege contra duplicatas.

---

### MC05: Registrar Decisão

**Propósito:** Registra ADRs (Architecture Decision Records) rapidamente.

**Shortcut:**
```
1. Ask for Input (Text): "Qual a decisão?" → save as "decision"
2. Ask for Input (Text): "Justificativa?" → save as "rationale"
3. Ask for Input (Text): "Contexto (projeto/componente)?" → save as "context"
4. Get Current Date → format "yyyy-MM-dd" → save as "today"
5. Run Scriptable "MCP Call" (search_nodes com "[decision]")
6. If exists → Alert + Stop
7. Run Scriptable "MCP Call" with parameter:
   {
     "server": "memory",
     "tool": "create_entities",
     "arguments": {
       "entities": [{
         "name": "ADR: [decision]",
         "entity_type": "decision",
         "observations": [
           "[rationale]",
           "Contexto: [context]",
           "Data: [today]",
           "Status: aprovada"
         ]
       }]
     }
   }
8. Show Notification: "ADR registrada ✅"
```

---

### MC06: Atualizar Status

**Propósito:** Atualiza o status de uma entidade existente (corrige observation).

**Shortcut:**
```
1. Ask for Input (Text): "Nome da entidade?" → save as "entityName"
2. Run Scriptable "MCP Call" (open_nodes com "[entityName]")
3. Show current observations in Quick Look
4. Choose from List: ["em andamento", "concluído", "bloqueado", "cancelado", "pausado"]
   → save as "newStatus"
5. Run Scriptable "MCP Call" (delete_observations):
   {
     "server": "memory",
     "tool": "delete_observations",
     "arguments": {
       "deletions": [{
         "entity_name": "[entityName]",
         "observations": ["Status: *"]
       }]
     }
   }
   ⚠️ Nota: o delete precisa do texto exato da observation antiga.
   Alternativa: extrair do resultado do passo 2 a observation que começa com "Status:"
6. Run Scriptable "MCP Call" (add_observations):
   {
     "server": "memory",
     "tool": "add_observations",
     "arguments": {
       "observations": [{
         "entity_name": "[entityName]",
         "contents": ["Status: [newStatus]"]
       }]
     }
   }
7. Show Notification: "[entityName] → [newStatus] ✅"
```

---

### MC07: Ver Grafo do Projeto

**Propósito:** Dump completo do projeto ativo — útil para auditoria.

**Shortcut:**
```
1. Run Scriptable "MCP Call" with parameter:
   {"server":"memory","tool":"read_graph","arguments":{}}
2. Save result to Files (iCloud/Shortcuts/graph-dump-[date].json)
3. Show Quick Look with result
```

---

### MC08: Adicionar Observação

**Propósito:** Adiciona fato a uma entidade existente sem criar nada novo.

**Shortcut:**
```
1. Ask for Input (Text): "Nome da entidade?" → save as "entityName"
2. Run Scriptable "MCP Call" (open_nodes "[entityName]")
3. If not found → Alert "Entidade não encontrada" → Stop
4. Ask for Input (Text): "Observação?" → save as "observation"
5. Run Scriptable "MCP Call":
   {
     "server": "memory",
     "tool": "add_observations",
     "arguments": {
       "observations": [{
         "entity_name": "[entityName]",
         "contents": ["[observation]"]
       }]
     }
   }
6. Show Notification: "Observação adicionada a [entityName] ✅"
```

---

### MC09: Criar Relação

**Propósito:** Liga duas entidades existentes.

**Shortcut:**
```
1. Ask for Input (Text): "Entidade origem?" → save as "from"
2. Ask for Input (Text): "Entidade destino?" → save as "to"
3. Choose from List: ["uses", "depends_on", "delivers", "manages", "implements",
   "validates", "enables_access_to", "sits_behind", "aggregates", "defines_architecture_of"]
   → save as "relationType"
4. Run Scriptable "MCP Call":
   {
     "server": "memory",
     "tool": "create_relations",
     "arguments": {
       "relations": [{
         "from": "[from]",
         "to": "[to]",
         "relation_type": "[relationType]"
       }]
     }
   }
5. Show Notification: "[from] → [relationType] → [to] ✅"
```

---

## Shortcuts — Outros MCPs

### TD01: Criar Tarefa Rápida (Todoist)

**Shortcut:**
```
1. Ask for Input (Text): "Tarefa?" → save as "task"
2. Ask for Input (Text): "Quando? (ex: today, tomorrow)" → save as "due"
3. Run Scriptable "MCP Call":
   {
     "server": "todoist",
     "tool": "createTask",
     "arguments": {
       "content": "[task]",
       "dueString": "[due]"
     }
   }
4. Show Notification: "Tarefa criada ✅"
```

---

### TD02: Minhas Tarefas de Hoje (Todoist)

**Shortcut:**
```
1. Run Scriptable "MCP Call":
   {
     "server": "todoist",
     "tool": "listTasks",
     "arguments": {"filter": "today"}
   }
2. Parse result → format task list
3. Show Result
```

---

### GH01: Meus Issues Abertos (GitHub)

**Shortcut:**
```
1. Ask for Input (Text): "Repo? (owner/repo)" → save as "repo"
2. Split "repo" by "/" → owner, repoName
3. Run Scriptable "MCP Call":
   {
     "server": "github",
     "tool": "list_issues",
     "arguments": {
       "owner": "[owner]",
       "repo": "[repoName]",
       "state": "open"
     }
   }
4. Show Result
```

---

### BS01: Busca Web Rápida (Brave Search)

**Shortcut:**
```
1. Ask for Input (Text): "Buscar o quê?" → save as "query"
2. Run Scriptable "MCP Call":
   {
     "server": "brave",
     "tool": "brave_web_search",
     "arguments": {"query": "[query]"}
   }
3. Parse result → extract titles + URLs
4. Choose from List → Open URL in Safari
```

---

## Debug — a-Shell mini

Para troubleshooting rápido direto do iPhone:

```bash
# Testar conectividade
curl -s -H "Authorization: Bearer TOKEN" \
  https://api.wagnerlima.cc/health

# Chamar MCP diretamente (initialize + tool call em sequência)
curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  https://api.wagnerlima.cc/servers/memory/mcp \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"ashell","version":"1.0"}}}'

# Depois com session id (se retornado):
curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TOKEN" \
  -H "Mcp-Session-Id: SESSION_ID" \
  https://api.wagnerlima.cc/servers/memory/mcp \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search_nodes","arguments":{"query":"Phase 4"}}}'
```

---

## Referência Rápida

| Shortcut | Ação | MCP | Tool |
|----------|------|-----|------|
| MC01 | Verificar projeto ativo | memory | get_current_project |
| MC02 | Trocar projeto | memory | list_projects + switch_project |
| MC03 | Buscar memória | memory | search_nodes |
| MC04 | Salvar ideia | memory | search_nodes + create_entities |
| MC05 | Registrar decisão (ADR) | memory | search_nodes + create_entities |
| MC06 | Atualizar status | memory | open_nodes + delete_observations + add_observations |
| MC07 | Ver grafo completo | memory | read_graph |
| MC08 | Adicionar observação | memory | open_nodes + add_observations |
| MC09 | Criar relação | memory | create_relations |
| TD01 | Criar tarefa | todoist | createTask |
| TD02 | Tarefas de hoje | todoist | listTasks |
| GH01 | Issues abertos | github | list_issues |
| BS01 | Busca web | brave | brave_web_search |

---

## Dicas de Integridade

1. **Sempre MC01 antes de MC04/MC05** — confirme o projeto certo
2. **MC04 e MC05 fazem search before create** — anti-duplicata built-in
3. **MC06 precisa do texto exato** — o `delete_observations` é por match exato de string
4. **Prefira MC08 a criar entidade nova** — se a entidade já existe, adicione observação
5. **MC07 periódico** — faça dump do grafo semanalmente para backup local no iCloud

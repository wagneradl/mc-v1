# Memory Cloud — Guia de Uso

Manual prático para usar o sistema de memória persistente do Memory Cloud com assistentes AI (Claude, Cursor, ChatGPT, etc).

---

## Conceitos fundamentais

O Memory Cloud armazena conhecimento como um **grafo** com três elementos:

- **Entidades** — nós do grafo. Representam qualquer coisa: pessoas, tecnologias, projetos, decisões, bugs. Cada entidade tem um `name`, um `entity_type` e zero ou mais observações.
- **Observações** — fatos ou notas ligados a uma entidade. São a "memória" em si. Você não recria entidades para adicionar informação — adiciona observações.
- **Relações** — arestas direcionadas entre entidades. Sempre em voz ativa: A `uses` B, A `depends_on` B, A `manages` B.

Tudo vive dentro de **projetos**, que são bancos SQLite isolados. Cada projeto tem seu próprio grafo, completamente separado dos outros.

---

## Fluxo básico de trabalho

```
1. Criar ou trocar pra um projeto
2. Registrar entidades com observações iniciais
3. Conectar entidades com relações
4. Buscar conhecimento quando precisar
5. Evoluir com novas observações ao longo do tempo
```

---

## Tools disponíveis (16 total)

### Gestão de projetos (7)

| Tool | O que faz |
|------|-----------|
| `list_projects` | Lista projetos (filtro: active, archived, all) |
| `create_project` | Cria novo projeto com banco isolado |
| `switch_project` | Troca o projeto ativo da sessão |
| `get_current_project` | Mostra qual projeto está ativo |
| `archive_project` | Arquiva (move .db pra archive/, preserva dados) |
| `restore_project` | Restaura projeto arquivado |
| `delete_project` | Exclui permanentemente (irreversível) |

### Knowledge graph (9)

| Tool | O que faz |
|------|-----------|
| `create_entities` | Cria entidades com tipo e observações iniciais |
| `add_observations` | Adiciona fatos novos a entidades existentes |
| `create_relations` | Cria conexões direcionadas entre entidades |
| `search_nodes` | Busca full-text (FTS5) em nomes e observações |
| `open_nodes` | Busca entidades por nome exato |
| `read_graph` | Retorna o grafo inteiro do projeto ativo |
| `delete_entities` | Soft delete (marca deleted_at, não apaga) |
| `delete_observations` | Remove observações específicas |
| `delete_relations` | Remove relações específicas |

> Todas as tools de knowledge graph exigem um projeto ativo (`switch_project` primeiro).

---

## Exemplos práticos por cenário

### 1. Início de um novo projeto de consultoria

**Prompt:**
> Crie um projeto "cliente-acme" no Memory Cloud e registre o contexto inicial: o cliente é a ACME Corp, setor de logística, stack principal é Java + AWS, o sponsor é João Silva (CTO), e o objetivo do projeto é migrar de monolito para microsserviços.

**O que o AI faz:**
```
create_project("cliente-acme", "Projeto de consultoria ACME Corp — migração para microsserviços")
switch_project("cliente-acme")
create_entities([
  { name: "ACME Corp", type: "organization", observations: ["Setor: logística", "Stack: Java + AWS", "Desafio: migração monolito → microsserviços"] },
  { name: "João Silva", type: "person", observations: ["CTO da ACME Corp", "Sponsor do projeto de migração"] },
  { name: "Migração Microsserviços", type: "project", observations: ["Objetivo: decompor monolito Java em microsserviços", "Infraestrutura alvo: AWS ECS/EKS"] }
])
create_relations([
  { from: "João Silva", to: "ACME Corp", relation_type: "works_at" },
  { from: "João Silva", to: "Migração Microsserviços", relation_type: "sponsors" },
  { from: "Migração Microsserviços", to: "ACME Corp", relation_type: "benefits" }
])
```

### 2. Registrar uma decisão técnica (ADR)

**Prompt:**
> Registra no Memory Cloud que decidimos usar RabbitMQ ao invés de Kafka pro projeto cliente-acme. O motivo foi a equipe já ter experiência com RabbitMQ e o volume de mensagens não justificar Kafka. A alternativa rejeitada foi AWS SQS por lock-in.

**O que o AI faz:**
```
switch_project("cliente-acme")
create_entities([{
  name: "ADR: RabbitMQ como message broker",
  type: "decision",
  observations: [
    "Escolhido RabbitMQ para comunicação entre microsserviços",
    "Motivo: equipe já tem experiência, volume de mensagens é moderado",
    "Alternativa considerada: Kafka (rejeitada — overengineering pro volume)",
    "Alternativa considerada: AWS SQS (rejeitada — vendor lock-in)",
    "Data: 2026-02-08"
  ]
}])
create_relations([
  { from: "ADR: RabbitMQ como message broker", to: "Migração Microsserviços", relation_type: "defines_architecture_of" }
])
```

### 3. Registrar um bug ou lição aprendida

**Prompt:**
> Registra que descobrimos um bug no deploy: o health check do ECS tava com timeout de 5s mas o Spring Boot leva 12s pra iniciar. A solução foi aumentar pra 30s e adicionar um startupProbe.

**O que o AI faz:**
```
create_entities([{
  name: "Bug: ECS health check timeout",
  type: "lesson",
  observations: [
    "Health check padrão do ECS: 5s timeout",
    "Spring Boot startup time: ~12s",
    "Sintoma: tasks sendo killed antes de ficar ready",
    "Solução: aumentar timeout para 30s + startupProbe separado",
    "Lição: sempre medir startup time antes de configurar health checks"
  ]
}])
create_relations([
  { from: "Bug: ECS health check timeout", to: "Migração Microsserviços", relation_type: "discovered_in" }
])
```

### 4. Buscar conhecimento antes de agir

**Prompt:**
> O que já sabemos sobre health checks no projeto cliente-acme?

**O que o AI faz:**
```
switch_project("cliente-acme")
search_nodes("health check")
→ Encontra a entidade "Bug: ECS health check timeout" com todas as observações e relações
```

**Prompt (busca mais ampla):**
> Me dá um panorama de todas as decisões técnicas do projeto ACME.

**O que o AI faz:**
```
search_nodes("decision")  // busca por tipo
read_graph()              // ou lê o grafo inteiro se o projeto for pequeno
```

### 5. Evoluir conhecimento existente

**Prompt:**
> Atualiza a entidade do João Silva — ele foi promovido pra VP de Engineering e agora o novo sponsor do projeto é a Maria Souza que assumiu como CTO.

**O que o AI faz:**
```
add_observations([
  { entity_name: "João Silva", contents: ["Promovido a VP of Engineering em 2026-02", "Não é mais sponsor direto do projeto de migração"] }
])
create_entities([{
  name: "Maria Souza",
  type: "person",
  observations: ["Nova CTO da ACME Corp (substituiu João Silva)", "Assumiu como sponsor do projeto de migração"]
}])
create_relations([
  { from: "Maria Souza", to: "ACME Corp", relation_type: "works_at" },
  { from: "Maria Souza", to: "Migração Microsserviços", relation_type: "sponsors" }
])
delete_relations([
  { from: "João Silva", to: "Migração Microsserviços", relation_type: "sponsors" }
])
```

### 6. Onboarding (nova conversa ou novo assistente)

**Prompt:**
> Carrega o contexto do projeto cliente-acme pra eu te atualizar sobre o que estamos fazendo.

**O que o AI faz:**
```
switch_project("cliente-acme")
read_graph()
→ Retorna todas as entidades, observações e relações de uma vez
→ O AI tem o mapa completo do projeto instantaneamente
```

### 7. Gestão do ciclo de vida

**Prompt:**
> O projeto da ACME terminou. Arquiva mas não apaga.

```
archive_project("cliente-acme")
→ Move o .db para archive/, status = "archived"
→ Dados preservados, projeto não aparece em list_projects(status="active")
```

**Prompt (6 meses depois):**
> A ACME voltou pedindo fase 2. Restaura o projeto.

```
restore_project("cliente-acme")
→ Move de volta para projects/, status = "active"
→ Todo o grafo intacto
```

---

## Tipos de entidade recomendados

Não há restrição de tipos — use o que fizer sentido. Estes são os mais úteis:

| Tipo | Quando usar | Exemplo |
|------|-------------|---------|
| `person` | Pessoas relevantes | "João Silva", "Maria Souza" |
| `organization` | Empresas, times | "ACME Corp", "Squad Payments" |
| `project` | Iniciativas, workstreams | "Migração Microsserviços" |
| `component` | Componentes técnicos | "API Gateway", "Auth Service" |
| `technology` | Ferramentas e libs | "RabbitMQ", "Spring Boot" |
| `decision` | ADRs, escolhas técnicas | "ADR: RabbitMQ como broker" |
| `lesson` | Bugs, aprendizados | "Bug: ECS health check timeout" |
| `concept` | Ideias, padrões | "Event Sourcing", "CQRS" |
| `document` | Referência a docs | "RFC: API v2 Design" |
| `milestone` | Marcos do projeto | "Go-live fase 1" |

---

## Convenções para relações

Relações são arestas direcionadas. Use **voz ativa** e nomes descritivos:

| Relação | Significado | Exemplo |
|---------|-------------|---------|
| `uses` | Usa/depende de | Memory MCP `uses` ncruces/go-sqlite3 |
| `works_at` | Trabalha em | João `works_at` ACME |
| `sponsors` | Patrocina/é responsável | Maria `sponsors` Migração |
| `manages` | Gerencia | Tech Lead `manages` Squad |
| `depends_on` | Depende de | Service A `depends_on` Service B |
| `hosts` | Hospeda | VPS `hosts` Caddy |
| `affects` | Impacta/influencia | ADR `affects` Component |
| `discovered_in` | Encontrado em | Bug `discovered_in` Service |
| `defines_architecture_of` | Define arquitetura | ADR `defines_architecture_of` Project |
| `replaces` | Substitui | Novo `replaces` Antigo |
| `blocks` | Bloqueia | Issue `blocks` Feature |
| `part_of` | Faz parte de | Module `part_of` System |

---

## Dicas de busca (FTS5)

O `search_nodes` usa SQLite FTS5. Algumas dicas:

```
search_nodes("health check")     → busca por tokens separados (OR implícito)
search_nodes("health AND check") → ambos os termos devem existir
search_nodes("migra*")           → prefixo — encontra "migração", "microsserviços", etc
search_nodes("bug OR lesson")    → qualquer um dos termos
search_nodes("NOT kafka")        → excluir termo
```

A busca cruza **nomes de entidades** e **conteúdo de observações** ao mesmo tempo, retornando entidades completas com observações e relações.

Para busca exata por nome, use `open_nodes(["Nome Exato"])` — mais rápido e preciso.

---

## Padrões de uso em diferentes clientes

### Claude Desktop (claude.ai)

Os tools aparecem automaticamente no painel. Basta pedir em linguagem natural:

> "Troca pro projeto X e me mostra o que tem lá"
> "Registra que decidimos usar PostgreSQL ao invés de MySQL"
> "O que sabemos sobre deploy nesse projeto?"

### Claude Code (terminal)

Mesmo endpoint, acesso direto via HTTP:

```bash
# Os tools ficam disponíveis automaticamente após configurar em settings
claude "switch to project memory-cloud and show me the architecture decisions"
```

### Cursor / Windsurf / ChatGPT

Configurar via SSE endpoint. Os tools aparecem como function calls:

```
Endpoint: https://api.wagnerlima.cc/servers/memory/sse
Header: Authorization: Bearer <token>
```

---

## Quando usar cada operação

```
Preciso documentar algo novo?
├─ É um conceito/pessoa/tech nova? → create_entities
├─ É info sobre algo que já existe? → add_observations
└─ É uma conexão entre coisas? → create_relations

Preciso consultar?
├─ Sei o nome exato? → open_nodes
├─ Quero buscar por tema? → search_nodes
└─ Quero ver tudo? → read_graph

Preciso organizar?
├─ Remover entidade? → delete_entities (soft delete)
├─ Remover observação? → delete_observations
├─ Remover relação? → delete_relations
├─ Pausar projeto? → archive_project
├─ Retomar projeto? → restore_project
└─ Eliminar de vez? → delete_project (irreversível!)
```

---

## Limites e considerações

- **Tamanho do grafo:** SQLite lida bem com milhares de entidades por projeto. FTS5 busca em ~140ms com 1M de registros. Não é um gargalo prático.
- **Concorrência:** WAL mode permite leituras paralelas. Escritas são serializadas (single-user, não é problema).
- **Backup:** Diário automático às 03:00 UTC com retenção de 30 dias. Para backup manual: `ssh deploy@api.wagnerlima.cc` e executar `./backup.sh`.
- **Isolamento:** Projetos são bancos separados no filesystem. Não há como um projeto acessar dados de outro.
- **Soft delete:** Entidades, observações e relações deletadas ficam marcadas com `deleted_at` — invisíveis nas buscas, mas recuperáveis por acesso direto ao SQLite.

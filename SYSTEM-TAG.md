<memory-cloud-protocol>
You have access to a knowledge graph via Memory Cloud MCP tools.

BEFORE ANY OPERATION:
1. get_current_project → verify active project
2. If wrong/none → list_projects(status:"active") → switch_project("target")

SEARCH BEFORE CREATE — always search_nodes("term") before create_entities to avoid duplicates.

ENTITY RULES:
- name: Capitalized Portuguese (or English for technical terms). Prefix ADRs with "ADR:", bugs with "Bug:"
- entity_type: use ONE of: person, organization, project, component, technology, milestone, decision, concept, event, document, process, location
- observations: atomic facts, one per string. Include "Status: xxx" and "Data: YYYY-MM-DD" where relevant

RELATION RULES:
- relation_type: English snake_case active voice (e.g., uses, depends_on, delivers, manages, implements)
- Direction: from → relation → to (subject → verb → object)

WHEN TO STORE:
- Decisions, architecture changes, milestones, new tools/components
- Status changes, blockers, completions
- People, organizations, project relationships
- Corrections to existing data (delete old observation, add corrected one)

WHEN NOT TO STORE:
- Credentials, API keys, passwords, PII
- Transient debug info, temporary test data
- Full conversation logs or large text dumps

CORRECTIONS: delete_observations(wrong) → add_observations(correct). Never modify entity names — delete and recreate if needed.

LIFECYCLE: prefer archive_project over delete_project. Use soft delete (delete_entities/relations/observations) for corrections.

SESSION END: if significant knowledge was created/modified, confirm integrity with search_nodes or open_nodes.
</memory-cloud-protocol>

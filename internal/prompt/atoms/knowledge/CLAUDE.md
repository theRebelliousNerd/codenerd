# knowledge/ - Knowledge Management Atoms

Guidance for knowledge extraction, storage, and retrieval.

## Files

| File | Purpose |
|------|---------|
| `extraction.yaml` | Extracting knowledge atoms from content |
| `persistence.yaml` | Storing knowledge in memory tiers |
| `retrieval.yaml` | Semantic knowledge retrieval |
| `encyclopedia.yaml` | Knowledge system reference |

## Memory Tiers

| Tier | Storage | Use Case |
|------|---------|----------|
| RAM | In-memory | Working facts |
| Vector | SQLite + embeddings | Semantic search |
| Graph | knowledge_graph table | Relationships |
| Cold | cold_storage table | Learned patterns |

## Knowledge Atoms

Extracted knowledge is stored as:
```text
knowledge_atom(ID, Category, Content, Source, Confidence, Timestamp)
```

## Selection

Knowledge atoms are selected for researcher and librarian shards.


> *[Archived & Reviewed by The Librarian on 2026-01-25]*
# Getting Started with CodeNERD for Python

This guide will walk you through setting up CodeNERD for a Python project (like ArangoRDF). CodeNERD treats your code as a "Glass Box," indexing it into a knowledge graph and allowing it to safely execute code in sandboxed Docker containers.

## Prerequisites

1.  **CodeNERD CLI Installed**: Ensure you have built the `nerd` binary.
2.  **Docker Running**: CodeNERD uses Docker to spin up ephemeral environments for your Python code.
3.  **Python Project**: A standard Python project with `requirements.txt`, `setup.py`, or `pyproject.toml`.

## 1. Initialization

Navigate to your Python project root.

```bash
cd ~/my-python-project
nerd init
```

**What happens:**
*   **`.nerd/` directory created**: Stores the local database and configuration.
*   **Indexing**: CodeNERD parses all your `.py` files using Tree-sitter to understand classes, functions, and imports.
*   **Fact Generation**: It creates a highly detailed "symbol graph" of your code.
*   **Vector Embeddings**: It generates embeddings for your code to enable semantic search.

## 2. Using the Interactive Agent

Launch the TUI (Text User Interface):

```bash
nerd
```

You are now in a chat session with the agent. The agent has context of your entire codebase.

### Example Workflows

#### A. Explaining Code
**You:** "Explain how the `GraphConnection` class works and where it is instantiated."
**Agent:** Will query its internal knowledge graph (Mangle) to find symbols and cross-references, then explain it to you.

#### B. Refactoring
**You:** "Rename the `connect` method to `establish_connection` and update all callers."
**Agent:**
1.  Identifies all references.
2.  Proposes a plan.
3.  Generates a diff.
4.  Asks for your approval (`/approve`).

#### C. Running Tests (The "Tactile" Layer)
**You:** "Run the tests for the connection module."
**Agent:**
1.  Spins up a **Docker container** mirroring your environment.
2.  Installs dependencies (from `requirements.txt`).
3.  Executes `pytest` inside the container.
4.  Reports usage and results back to you.

## 3. Sandboxed Execution

One of CodeNERD's strongest features for Python is **Sandboxed Execution**. It doesn't run code on your host machine.

When you ask it to "Run tests" or "Verify this fix":
1.  It creates a Docker container tagged `nerd-python-<repo>`.
2.  It mounts your code (or clones it).
3.  It creates a virtual environment inside the container.
4.  It executes the command.

This means you can let the agent iterate on fixes without worrying about it messing up your local environment.

## Troubleshooting

**"No tests found"**
*   Ensure you have `pytest` installed in your project.
*   Check that your test files usually follow `test_*.py`.

**"Docker error"**
*   Make sure Docker Desktop (or engine) is running.
*   Run `docker ps` to see if CodeNERD containers are active.

> *[Archived & Reviewed by The Librarian on 2026-01-28]*

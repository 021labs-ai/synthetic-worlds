# Synthetic Worlds

Realistic tool mocking powered by LLMs and production traces. Stop writing mocks by hand — import your real traces and get stateful, coherent responses that match how your tools actually behave.

## How it works

1. **Import traces** from Langfuse, LangSmith, or upload manually
2. **Create a world** — an isolated environment that remembers state across calls
3. **Register your tools** and call them — the LLM generates realistic responses grounded in your real production data

```python
from synthetic_worlds import create_synthetic_world

with create_synthetic_world() as world:
    @world.tool
    def get_user(user_id: str) -> dict:
        """Fetch a user by ID."""
        ...

    @world.tool
    def list_tickets(user_id: str) -> list:
        """List support tickets for a user."""
        ...

    user = get_user("u1")        # {"id": "u1", "name": "Alice", "tier": "enterprise"}
    tickets = list_tickets("u1") # [{"id": "t1", "subject": "Billing issue", "status": "open"}]
    # The user and tickets are consistent — Alice is remembered across calls
```

## Quickstart

### 1. Start the server

```bash
# Set your LLM API key
export ANTHROPIC_API_KEY=sk-ant-...

# Start with Docker Compose
docker compose up -d
```

This starts 3 containers: the API server (port 7878), PostgreSQL, and Redis.

### 2. Import traces

The value of Synthetic Worlds comes from grounding responses in real production data.

```bash
# Import traces from a JSON file
curl -X POST http://localhost:7878/v1/traces/import \
  -H "Authorization: Bearer change-me-in-production" \
  -H "Content-Type: application/json" \
  -d '{
    "traces": [
      {
        "name": "customer-support-flow",
        "spans": [
          {"name": "get_user", "input": {"user_id": "u1"}, "output": {"id": "u1", "name": "Alice", "tier": "enterprise"}},
          {"name": "list_tickets", "input": {"user_id": "u1"}, "output": [{"id": "t1", "subject": "Billing issue"}]}
        ]
      }
    ]
  }'
```

Or import from observability platforms:

```python
from synthetic_worlds.importers import import_langfuse, import_from_file

# From Langfuse export
import_langfuse("langfuse-export.json", api_key="change-me-in-production")

# From a JSON file
import_from_file("traces.json", api_key="change-me-in-production")
```

### 3. Install the Python SDK

```bash
pip install synthetic-worlds
```

### 4. Create a world and mock your tools

```python
from synthetic_worlds import create_synthetic_world

world = create_synthetic_world(
    api_key="change-me-in-production",
    mode="stateful",  # maintains state across calls
)

@world.tool
def get_user(user_id: str) -> dict:
    """Fetch a user by ID."""
    ...

@world.tool
def create_ticket(user_id: str, subject: str, priority: str) -> dict:
    """Create a support ticket."""
    ...

# These return realistic, stateful responses
user = get_user("u1")
ticket = create_ticket("u1", "Login broken", "high")

world.close()
```

## Features

- **Stateful** — entities created in step 1 are remembered in step 3
- **Grounded in production** — responses match your real tool behavior via imported traces
- **Deterministic** — set a seed for reproducible outputs
- **Error injection** — simulate timeouts, rate limits, and failures
- **Any SDK** — register tools from OpenAI, Anthropic, or plain functions
- **BYOK** — bring your own Anthropic, OpenAI, or xAI API key

## Configuration

All configuration via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | — | PostgreSQL connection string |
| `REDIS_URL` | `redis://localhost:6379/0` | Redis connection string |
| `SYNTH_API_KEY` | — | API key for authentication |
| `ANTHROPIC_API_KEY` | — | Your Anthropic API key |
| `OPENAI_API_KEY` | — | Your OpenAI API key (optional) |
| `XAI_API_KEY` | — | Your xAI API key (optional) |
| `DEFAULT_MODEL` | `claude-sonnet-4-6` | Default LLM model |
| `PORT` | `7878` | Server port |
| `WORLD_TTL_SECONDS` | `3600` | World expiration time |

## API

### Synthetic Worlds

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/synthetic/worlds` | Create a world |
| `POST` | `/v1/synthetic/call` | Execute a tool call |
| `POST` | `/v1/synthetic/worlds/:id/reset` | Reset a world |
| `DELETE` | `/v1/synthetic/worlds/:id` | Close a world |
| `GET` | `/v1/synthetic/worlds` | List worlds |
| `GET` | `/v1/synthetic/worlds/:id` | Get world details |
| `GET` | `/v1/synthetic/worlds/:id/state` | Get world state |
| `GET` | `/v1/synthetic/worlds/:id/calls` | List world calls |
| `GET` | `/v1/synthetic/calls` | List all calls |

### Trace Import

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/v1/traces/import` | Import traces (native format) |
| `POST` | `/v1/traces/import/langfuse` | Import from Langfuse |
| `POST` | `/v1/traces/import/langsmith` | Import from LangSmith |

## Modes

- **`schema_only`** — generates responses from tool schema only
- **`examples`** — adds real examples from imported traces
- **`stateful`** (default) — examples + accumulated state tracking across calls

## Development

```bash
# Build
make build

# Run locally
make run

# Run tests
make test

# Docker
make docker-up
make docker-down
```

## License

Apache 2.0

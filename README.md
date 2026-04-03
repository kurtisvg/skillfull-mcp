# skillful-mcp

An MCP middleware that aggregates multiple downstream MCP servers into
mcp-native Agent Skills. Each server becomes a Skill that an AI agent can
discover and execute those tools through code mode.

## Why

MCP servers solve connectivity — any tool can expose a standard interface. But
connecting an agent to many servers creates a new problem: [tool
bloat](https://kvg.dev/posts/20260125-skills-and-mcp/).

An agent with access to 5 MCP servers might have 80+ tools. Every tool schema
gets loaded into the context window before the user says a word. The model's
attention is diluted across dozens of options, accuracy drops, and latency
increases. Adding more capabilities makes the agent worse.

skillful-mcp fixes this through **progressive disclosure**. Instead of injecting
all tool definitions upfront, the agent sees just 4 tools (`list_skills`,
`use_skill`, `read_resource`, `execute_code`). It discovers specific tool
schemas on-demand by calling `use_skill`, keeping the context window lean. This
collapses thousands of tokens of tool definitions down to a lightweight index —
and only loads what's needed, when it's needed.

## How it works

```
Agent  <--MCP-->  skillful-mcp  <--MCP-->  Database Server
                                <--MCP-->  Filesystem Server
                                <--MCP-->  API Server
```

skillful-mcp reads a standard `mcp.json` config (same format as Claude Code /
Claude Desktop), connects to each downstream server, and exposes four tools:

| Tool | Description |
|------|-------------|
| `list_skills` | Returns the names of all configured downstream servers |
| `use_skill` | Lists the tools and resources available in a specific skill |
| `read_resource` | Reads a resource from a specific skill |
| `execute_code` | Runs Python code in a secure [Monty](https://github.com/pydantic/monty) sandbox |

The typical agent workflow is:

1. Call `list_skills` to see what's available
2. Call `use_skill` to inspect a skill's tools and their input schemas
3. Use `execute_code` to orchestrate tool calls in a single round-trip

### Code mode example

After discovering tools via `use_skill`, the agent can call them directly by
name inside `execute_code`:

```python
# Call tools from different skills in a single execution
users = query(sql="SELECT name, email FROM users WHERE active = true")
report = read_file(path="/templates/report.md")
users + "\n\n" + report
```

All downstream tools are available as functions with positional and keyword
arguments. If two skills define a tool with the same name, the function is
prefixed with the skill name (e.g. `database_search`, `docs_search`). Tool
names returned by `use_skill` always match the function names in `execute_code`.

## Configuration

Create an `mcp.json` file:

```json
{
  "mcpServers": {
    "<mcp-name>": { ... }
  }
}
```

Each entry in `mcpServers` is a downstream server that becomes a skill. The key
is the skill name. The value depends on the transport type:

### STDIO server

Spawns the server as a child process. Only env vars explicitly listed in `env`
are passed to the child — the parent environment is not inherited.

| Field | Required | Description |
|-------|----------|-------------|
| `command` | yes | Executable to run |
| `args` | no | Arguments array |
| `env` | no | Environment variables for the child process |

```json
{
  "mcpServers": {
    "database": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-sqlite", "mydb.db"],
      "env": {
        "PATH": "/usr/local/bin:/usr/bin"
      }
    }
  }
}
```

### HTTP server

Connects via Streamable HTTP.

| Field | Required | Description |
|-------|----------|-------------|
| `type` | yes | Must be `"http"` |
| `url` | yes | Server endpoint URL |
| `headers` | no | HTTP headers (e.g. auth tokens) |

```json
{
  "mcpServers": {
    "remote-api": {
      "type": "http",
      "url": "https://api.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${API_KEY}"
      }
    }
  }
}
```

### SSE server

Connects via Server-Sent Events.

| Field | Required | Description |
|-------|----------|-------------|
| `type` | yes | Must be `"sse"` |
| `url` | yes | SSE endpoint URL |
| `headers` | no | HTTP headers |

## Running

### Build and run

```sh
go build -o skillful-mcp .
./skillful-mcp --config mcp.json
```

### Run directly

```sh
go run . --config mcp.json
go run . --config mcp.json --transport http --port 8080
### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `./mcp.json` | Path to the config file |
| `--transport` | `stdio` | Upstream transport: `stdio` or `http` |
| `--host` | `localhost` | HTTP listen host |
| `--port` | `8080` | HTTP listen port |

### Use with MCP clients

**Gemini CLI** (`~/.gemini/settings.json`):

```json
{
  "mcpServers": {
    "skillful": {
      "command": "./skillful-mcp",
      "args": ["--config", "/path/to/mcp.json"]
    }
  }
}
```

**Claude Code** (`.claude/settings.json`):

```json
{
  "mcpServers": {
    "skillful": {
      "command": "./skillful-mcp",
      "args": ["--config", "/path/to/mcp.json"]
    }
  }
}
```

**Codex CLI** (`~/.codex/config.toml`):

```toml
[mcp_servers.skillful]
command = "./skillful-mcp"
args = ["--config", "/path/to/mcp.json"]
```

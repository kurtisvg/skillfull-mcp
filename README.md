# skillful-mcp

[![Go](https://img.shields.io/github/go-mod/go-version/kurtisvg/skillful-mcp)](https://go.dev/)
[![CI](https://github.com/kurtisvg/skillful-mcp/actions/workflows/test.yml/badge.svg)](https://github.com/kurtisvg/skillful-mcp/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/kurtisvg/skillful-mcp)](https://goreportcard.com/report/github.com/kurtisvg/skillful-mcp)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Too many MCP tools slowing your agent down? Might be a Skill Issue 😉

**skillful-mcp** eliminates tool bloat by turning your MCP servers into Agent Skills in an MCP-native way.

- 🔍 **Progressive Disclosure** — start with 4 tools, discover more as needed
- ⚡ **Code Mode** — trigger and combine multiple tool calls with Python
- 🔒 **Secure sandbox** — code executes in a sandbox, not your shell
- 🔌 **Any MCP client** — works with Gemini CLI, Claude Code, Codex, and more

## Table of contents

- [Why?](#-why)
- [How it works](#-how-it-works)
- [Getting started](#-getting-started)
- [Configuration](#-configuration)

## ❓ Why?

Connecting an agent to too many tools (or MCP servers) creates
[tool bloat][tool-bloat]. An agent with access to 5 servers might have 80+ tools
loaded into its context window before the user says a word. Accuracy drops,
latency increases, and adding capabilities makes the agent worse.

skillful-mcp fixes this through **progressive disclosure**. The agent sees just 4
tools and discovers specific schemas on-demand, collapsing thousands of tokens
down to a lightweight index.

[tool-bloat]: https://kvg.dev/posts/20260125-skills-and-mcp/

## 💡 How it works

```
Agent  <--MCP-->  skillful-mcp  <--MCP-->  Database Server
                                <--MCP-->  Filesystem Server
                                <--MCP-->  API Server
```

skillful-mcp reads a standard `mcp.json` config, connects to each downstream
server, and exposes four tools:

| Tool             | Description                                                                      |
|------------------|----------------------------------------------------------------------------------|
| `list_skills`    | Returns the names of all configured downstream servers                           |
| `use_skill`      | Lists the tools and resources available in a specific skill                      |
| `read_resource`  | Reads a resource from a specific skill                                           |
| `execute_code`   | Runs Python code in a secure [Monty](https://github.com/pydantic/monty) sandbox |

The typical agent workflow:

1. Call `list_skills` to see what's available
2. Call `use_skill` to inspect a skill's tools and their input schemas
3. Use `execute_code` to orchestrate tool calls in a single round-trip

### Example Code Mode Usage

After discovering tools via `use_skill`, the agent can call them directly by
name inside `execute_code` — chaining outputs from one tool into another:

```python
# Query users, then send each one a welcome email
users = query(sql="SELECT name, email FROM users WHERE welcomed = false")
for user in users:
    send_email(to=user["email"], subject="Welcome!", body="Hi " + user["name"])
"Sent " + str(len(users)) + " welcome emails"
```

All downstream tools are available as functions with positional and keyword
arguments. If two skills define a tool with the same name, the function is
prefixed with the skill name (e.g. `database_search`, `docs_search`). Tool
names returned by `use_skill` always match the function names in `execute_code`.

## 🚀 Getting started

### Install

<details open>
<summary><strong>Download a binary</strong></summary>

```sh
VERSION="0.0.1"
OS="linux"       # or: darwin, windows
ARCH="amd64"     # or: arm64

curl -L "https://github.com/kurtisvg/skillful-mcp/releases/download/v${VERSION}/skillful-mcp_${VERSION}_${OS}_${ARCH}" -o skillful-mcp
chmod +x skillful-mcp
```

Or download from the [releases page](https://github.com/kurtisvg/skillful-mcp/releases/latest).

</details>

<details>
<summary><strong>Docker</strong></summary>

```sh
docker run --rm \
  -v /path/to/mcp.json:/mcp.json \
  ghcr.io/kurtisvg/skillful-mcp:latest \
  --config /mcp.json --transport http --port 8080
```
</details>

<details>
<summary><strong>Go install</strong> (requires Go 1.25+)</summary>

```sh
go install github.com/kurtisvg/skillful-mcp@latest
```
</details>

<details>
<summary><strong>Build from source</strong></summary>

```sh
git clone https://github.com/kurtisvg/skillful-mcp.git
cd skillful-mcp
go build -o skillful-mcp .
```
</details>

### Create a config

Create an `mcp.json` file with your downstream servers:

```json
{
  "mcpServers": {
    "postgres": {
      "command": "npx",
      "args": ["-y", "@toolbox-sdk/server", "--prebuilt=postgres"],
      "description": "Postgres database tools — query, inspect schemas, and manage tables. Use when the user needs to read or write data, explore table structures, or run SQL.",
      "env": {
        "POSTGRES_HOST": "${POSTGRES_HOST}",
        "POSTGRES_USER": "${POSTGRES_USER}",
        "POSTGRES_PASSWORD": "${POSTGRES_PASSWORD}",
        "POSTGRES_DATABASE": "${POSTGRES_DATABASE}"
      }
    },
    "github-issues": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/x/issues",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      },
      "description": "GitHub issue management — create, search, update, and comment on issues. Use when the user mentions bugs, feature requests, or issue triage."
    }
  }
}
```

### Run

```sh
skillful-mcp --config mcp.json
```

Or over HTTP:

```sh
skillful-mcp --config mcp.json --transport http --port 8080
```

### Connect to your agent

<details>
<summary><strong>Gemini CLI</strong> (<code>~/.gemini/settings.json</code>)</summary>

```json
{
  "mcpServers": {
    "skillful": {
      "command": "/path/to/skillful-mcp",
      "args": ["--config", "/path/to/mcp.json"]
    }
  }
}
```
</details>

<details>
<summary><strong>Claude Code</strong> (<code>.claude/settings.json</code>)</summary>

```json
{
  "mcpServers": {
    "skillful": {
      "command": "/path/to/skillful-mcp",
      "args": ["--config", "/path/to/mcp.json"]
    }
  }
}
```
</details>

<details>
<summary><strong>Codex CLI</strong> (<code>~/.codex/config.toml</code>)</summary>

```toml
[mcp_servers.skillful]
command = "/path/to/skillful-mcp"
args = ["--config", "/path/to/mcp.json"]
```
</details>

Any MCP-compatible client works — just point it at the `skillful-mcp` binary.

### Advanced example: GitHub MCP Server

The [GitHub MCP server](https://github.com/github/github-mcp-server) exposes
19+ toolsets — a perfect candidate for skill decomposition. Instead of one
massive server, split it into focused skills by feature group. The agent sees
4 skills instead of 40+ tools, and calls `use_skill` only when it needs a
specific capability.

```json
{
  "mcpServers": {
    "github-issues": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/x/issues",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      },
      "description": "GitHub issue management — create, search, update, and comment on issues. Use when the user mentions bugs, feature requests, or issue triage."
    },
    "github-labels": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/x/labels",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      },
      "description": "GitHub label management — create, assign, and remove labels. Use when organizing or categorizing issues and pull requests."
    },
    "github-prs": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/x/pull_requests",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      },
      "description": "GitHub pull request workflows — review, merge, and manage PRs. Use when the user asks about code review, PR status, or merging changes."
    },
    "github-actions": {
      "type": "http",
      "url": "https://api.githubcopilot.com/mcp/x/actions",
      "headers": {
        "Authorization": "Bearer ${GITHUB_TOKEN}"
      },
      "description": "GitHub Actions CI/CD — trigger, monitor, and debug workflows. Use when the user asks about build status, failed checks, or re-running pipelines."
    }
  }
}
```

## 📝 Configuration

Each entry in `mcpServers` is a downstream server that becomes a skill. The key
is the skill name. The value depends on the transport type.

All string values support `${VAR}` environment variable expansion. Missing
variables cause a startup error.

### Common options

All server types support these optional fields:

| Field              | Description                                               |
|--------------------|-----------------------------------------------------------|
| `description`      | Override the server's instructions shown by `list_skills`  |
| `allowedTools`     | Only expose these tool names (default: all)                |
| `allowedResources` | Only expose these resource URIs (default: all)             |

Excluded tools are invisible everywhere — they won't appear in `use_skill`,
can't be called via `execute_code`, and won't cause name-conflict prefixing.

### STDIO server

Spawns the server as a child process. Only env vars explicitly listed in `env`
are passed to the child — the parent environment is not inherited.

| Field     | Required | Description                             |
|-----------|----------|-----------------------------------------|
| `command` | yes      | Executable to run                       |
| `args`    | no       | Arguments array                         |
| `env`     | no       | Environment variables for the child process |

```json
{
  "mcpServers": {
    "postgres": {
      "command": "npx",
      "args": ["-y", "@toolbox-sdk/server", "--prebuilt=postgres"],
      "description": "Postgres database tools — query, inspect schemas, and manage tables. Use when the user needs to read or write data, explore table structures, or run SQL.",
      "env": {
        "POSTGRES_HOST": "${POSTGRES_HOST}",
        "POSTGRES_USER": "${POSTGRES_USER}",
        "POSTGRES_PASSWORD": "${POSTGRES_PASSWORD}",
        "POSTGRES_DATABASE": "${POSTGRES_DATABASE}"
      }
    }
  }
}
```

### HTTP server

Connects via Streamable HTTP.

| Field     | Required | Description                     |
|-----------|----------|---------------------------------|
| `type`    | yes      | Must be `"http"`                |
| `url`     | yes      | Server endpoint URL             |
| `headers` | no       | HTTP headers (e.g. auth tokens) |

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

| Field     | Required | Description      |
|-----------|----------|------------------|
| `type`    | yes      | Must be `"sse"`  |
| `url`     | yes      | SSE endpoint URL |
| `headers` | no       | HTTP headers     |

### Flags

| Flag            | Default      | Description                           |
|-----------------|--------------|---------------------------------------|
| `--config`      | `./mcp.json` | Path to the config file               |
| `--transport`   | `stdio`      | Upstream transport: `stdio` or `http` |
| `--host`        | `localhost`  | HTTP listen host                      |
| `--port`        | `8080`       | HTTP listen port                      |
| `--version`     |              | Print version and exit                |

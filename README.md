# HATEOAS MCP Integration

Technical demonstration of integrating HATEOAS REST APIs with the Model Context Protocol (MCP).

## Components

### Bank API (`bank-api/`)
- **`hateoas-bank-api.go`**: Go HTTP server implementing HATEOAS bank account operations
- **`static/index.html`**: Browser client demonstrating dynamic link discovery and execution
- Serves on port 9001

### MCP Server (`mcp-server/`)
- **`mcp-server.py`**: Python MCP server that discovers and exposes HATEOAS endpoints as tools
- **`pyproject.toml`**: Python dependencies (httpx, mcp==1.0.0)
- Auto-generates MCP tools from `_links` metadata

## Architecture

The Go API returns JSON with embedded `_links` containing available operations:

```json
{
  "accountId": "acc-123",
  "balance": 1250.75,
  "_links": {
    "self": {"href": "/account", "method": "GET", "rel": "self"},
    "deposit": {"href": "/account/deposit", "method": "POST", "rel": "deposit"},
    "withdraw": {"href": "/account/withdraw", "method": "POST", "rel": "withdraw"}
  }
}
```

The MCP server polls these links and dynamically creates tools, sending `tools_changed` notifications when the API state changes (e.g., withdraw becomes unavailable at zero balance).

## Usage

```bash
# Terminal 1: Start Go API
cd bank-api && go run hateoas-bank-api.go

# Terminal 2: Start MCP server
cd mcp-server && python mcp-server.py

# Terminal 3: Connect MCP client
# Tools will appear/disappear based on current account state
```

Browser demo: http://localhost:9001/
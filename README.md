# monarch-money

MCP server for [Monarch Money](https://monarchmoney.com). Runs as a stdio binary — no network port, no auth.

## Build

```bash
make build
```

Requires `MONARCH_TOKEN` in `.env.local`. The token is baked into the binary at build time.

## Deploy to Windows

After building, copy the exe from the Linux build machine:

```powershell
scp kluu@192.168.50.242:/home/kluu/src/money/dist/monarch-mcp.exe C:\Users\Kevin\Documents\mcp\monarch-mcp.exe
```

## Claude Code config

```json
{
  "mcpServers": {
    "monarch-money": {
      "command": "C:\\Users\\Kevin\\Documents\\mcp\\monarch-mcp.exe"
    }
  }
}
```

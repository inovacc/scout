# @inovacc/scout-plugin-sdk

TypeScript SDK for building Scout plugins.

## Quick Start

```typescript
import { PluginServer, textResult } from '@inovacc/scout-plugin-sdk';

const server = new PluginServer();

server.registerTool('hello', async (params) => {
  return textResult(`Hello, ${params.name || 'world'}!`);
});

server.run();
```

## Plugin Manifest

Create `plugin.json`:
```json
{
  "name": "my-plugin",
  "version": "1.0.0",
  "capabilities": ["mcp_tool"],
  "tools": [{ "name": "hello", "description": "Say hello" }]
}
```

## Install

```bash
npm install @inovacc/scout-plugin-sdk
```

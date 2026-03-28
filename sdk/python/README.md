# scout-plugin-sdk

Python SDK for building Scout plugins.

## Quick Start

```python
from scout_plugin import PluginServer, text_result

server = PluginServer()

@server.register_tool("hello")
def hello(params):
    name = params.get("name", "world")
    return text_result(f"Hello, {name}!")

server.run()
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
pip install scout-plugin-sdk
```

"""JSON-RPC 2.0 plugin server for Scout."""

import json
import sys
from typing import Any, Callable, Dict, Optional


def text_result(text: str) -> dict:
    """Create a successful text tool result."""
    return {"content": [{"type": "text", "text": text}]}


def error_result(message: str) -> dict:
    """Create an error tool result."""
    return {"content": [{"type": "text", "text": message}], "isError": True}


class PluginServer:
    """JSON-RPC 2.0 server for Scout plugin communication."""

    def __init__(self) -> None:
        self._handlers: Dict[str, Callable] = {}

    def register_mode(self, handler: Callable) -> None:
        """Register a scraper mode handler."""
        self._handlers["mode/scrape"] = handler

    def register_extractor(self, handler: Callable) -> None:
        """Register an extractor handler."""
        self._handlers["extractor/extract"] = handler

    def register_tool(self, name: str, handler: Callable) -> None:
        """Register an MCP tool handler."""
        self._handlers[f"tool/{name}"] = handler

    def register_resource(self, uri: str, handler: Callable) -> None:
        """Register an MCP resource handler."""
        self._handlers["resource/read"] = handler

    def register_prompt(self, name: str, handler: Callable) -> None:
        """Register an MCP prompt handler."""
        self._handlers["prompt/get"] = handler

    def run(self) -> None:
        """Start the JSON-RPC server, reading from stdin and writing to stdout."""
        for line in sys.stdin:
            line = line.strip()
            if not line:
                continue

            try:
                req = json.loads(line)
            except json.JSONDecodeError:
                continue

            req_id = req.get("id")
            method = req.get("method", "")
            params = req.get("params", {})

            handler = self._handlers.get(method)
            if handler is None:
                self._respond(req_id, error={
                    "code": -32601,
                    "message": f"method not found: {method}",
                })
                continue

            try:
                result = handler(params)
                self._respond(req_id, result=result)
            except Exception as e:
                self._respond(req_id, error={
                    "code": -32000,
                    "message": str(e),
                })

    def _respond(
        self,
        req_id: Any,
        result: Any = None,
        error: Optional[dict] = None,
    ) -> None:
        resp: Dict[str, Any] = {"jsonrpc": "2.0", "id": req_id}
        if error:
            resp["error"] = error
        else:
            resp["result"] = result
        sys.stdout.write(json.dumps(resp) + "\n")
        sys.stdout.flush()

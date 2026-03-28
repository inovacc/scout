import * as readline from 'readline';

// JSON-RPC types
interface JsonRpcRequest {
  jsonrpc: '2.0';
  id: number | string;
  method: string;
  params?: Record<string, unknown>;
}

interface JsonRpcResponse {
  jsonrpc: '2.0';
  id: number | string;
  result?: unknown;
  error?: { code: number; message: string; data?: unknown };
}

interface JsonRpcNotification {
  jsonrpc: '2.0';
  method: string;
  params?: Record<string, unknown>;
}

// Tool result types
export interface ToolResult {
  content: Array<{ type: 'text'; text: string }>;
  isError?: boolean;
}

// Handler types
export type ModeHandler = (params: {
  target: string;
  options?: Record<string, unknown>;
}) => Promise<{ results: unknown[] }>;

export type ExtractorHandler = (params: {
  html: string;
  selector?: string;
}) => Promise<{ data: unknown }>;

export type ToolHandler = (params: Record<string, unknown>) => Promise<ToolResult>;

export type ResourceHandler = (params: { uri: string }) => Promise<{ content: string; mimeType: string }>;

export type PromptHandler = (params: {
  name: string;
  arguments?: Record<string, string>;
}) => Promise<{ messages: Array<{ role: string; content: string }> }>;

// Server
export class PluginServer {
  private handlers = new Map<string, (params: any) => Promise<unknown>>();
  private rl: readline.Interface | null = null;

  registerMode(handler: ModeHandler): void {
    this.handlers.set('mode/scrape', handler);
  }

  registerExtractor(handler: ExtractorHandler): void {
    this.handlers.set('extractor/extract', handler);
  }

  registerTool(name: string, handler: ToolHandler): void {
    this.handlers.set(`tool/${name}`, handler);
  }

  registerResource(uri: string, handler: ResourceHandler): void {
    this.handlers.set(`resource/read`, async (params) => {
      if (params.uri === uri) return handler(params);
      return { error: `resource not found: ${params.uri}` };
    });
  }

  registerPrompt(name: string, handler: PromptHandler): void {
    this.handlers.set(`prompt/get`, async (params) => {
      if (params.name === name) return handler(params);
      return { error: `prompt not found: ${params.name}` };
    });
  }

  async run(): Promise<void> {
    this.rl = readline.createInterface({ input: process.stdin });

    for await (const line of this.rl) {
      if (!line.trim()) continue;

      try {
        const req: JsonRpcRequest = JSON.parse(line);
        const handler = this.handlers.get(req.method);

        if (!handler) {
          this.respond(req.id, undefined, {
            code: -32601,
            message: `method not found: ${req.method}`,
          });
          continue;
        }

        try {
          const result = await handler(req.params || {});
          this.respond(req.id, result);
        } catch (err) {
          this.respond(req.id, undefined, {
            code: -32000,
            message: err instanceof Error ? err.message : String(err),
          });
        }
      } catch {
        // Invalid JSON — ignore
      }
    }
  }

  private respond(
    id: number | string,
    result?: unknown,
    error?: { code: number; message: string; data?: unknown }
  ): void {
    const resp: JsonRpcResponse = { jsonrpc: '2.0', id };
    if (error) resp.error = error;
    else resp.result = result;
    process.stdout.write(JSON.stringify(resp) + '\n');
  }
}

// Helper to create text tool results
export function textResult(text: string): ToolResult {
  return { content: [{ type: 'text', text }] };
}

export function errorResult(message: string): ToolResult {
  return { content: [{ type: 'text', text: message }], isError: true };
}

//! Thin local MCP bridge: stdio JSON-RPC server forwarding to the REST service.
//! Implements initialize, tools/list and tools/call with two tools:
//!   search_context {query}  -> POST /v1/search
//!   open_citation {citation} -> GET /v1/citations/{token}
//! The bridge adds no retrieval logic of its own; the service makes the one
//! access decision for both adapters.
//!
//! Environment:
//!   GANESH_REST_URL  base URL of the REST service, e.g. http://127.0.0.1:4315

use serde_json::{json, Value};
use std::io::{BufRead, Write};

const PROTOCOL_VERSION: &str = "2024-11-05";

fn main() {
    let base = std::env::var("GANESH_REST_URL").unwrap_or_else(|_| "http://127.0.0.1:4315".into());
    let stdin = std::io::stdin();
    let stdout = std::io::stdout();
    let mut out = stdout.lock();

    for line in stdin.lock().lines() {
        let Ok(line) = line else { break };
        if line.trim().is_empty() {
            continue;
        }
        let msg: Value = match serde_json::from_str(&line) {
            Ok(v) => v,
            Err(_) => continue,
        };
        let method = msg.get("method").and_then(Value::as_str).unwrap_or("");
        let id = msg.get("id").cloned();

        match method {
            "initialize" => {
                if let Some(id) = id {
                    respond(&mut out, id, json!({
                        "protocolVersion": PROTOCOL_VERSION,
                        "capabilities": { "tools": {} },
                        "serverInfo": { "name": "ganesh-mcp", "version": "0.1.0" }
                    }));
                }
            }
            "notifications/initialized" | "ping" => {}
            "tools/list" => {
                if let Some(id) = id {
                    respond(&mut out, id, json!({
                        "tools": [
                            {
                                "name": "search_context",
                                "description": "Search the Ganesh corpus. Returns up to three passages with citations, or explicit insufficient_context.",
                                "inputSchema": {
                                    "type": "object",
                                    "properties": {
                                        "query": { "type": "string" },
                                        "as_of": { "type": "string", "description": "Optional ISO date (YYYY-MM-DD) for a historical read." },
                                        "top_k": { "type": "integer", "description": "Passages to return (default 3, up to 15)." }
                                    },
                                    "required": ["query"]
                                }
                            },
                            {
                                "name": "open_citation",
                                "description": "Open a citation token returned by search_context. Resolves to the passage at the exact source revision, permission re-checked.",
                                "inputSchema": {
                                    "type": "object",
                                    "properties": { "citation": { "type": "string" } },
                                    "required": ["citation"]
                                }
                            }
                        ]
                    }));
                }
            }
            "tools/call" => {
                if let Some(id) = id {
                    let name = msg.pointer("/params/name").and_then(Value::as_str).unwrap_or("");
                    let args = msg.pointer("/params/arguments").cloned().unwrap_or(json!({}));
                    let result = call_tool(&base, name, &args);
                    respond(&mut out, id, result);
                }
            }
            _ => {
                if let Some(id) = id {
                    respond_error(&mut out, id, -32601, "method not found");
                }
            }
        }
    }
}

fn call_tool(base: &str, name: &str, args: &Value) -> Value {
    match name {
        "search_context" => {
            let query = args.get("query").and_then(Value::as_str).unwrap_or("");
            let mut body = json!({ "query": query });
            if let Ok(key) = std::env::var("GANESH_API_KEY") {
                if !key.is_empty() {
                    body["key"] = json!(key);
                }
            }
            if let Some(as_of) = args.get("as_of").and_then(Value::as_str) {
                body["as_of"] = json!(as_of);
            }
            if let Some(top_k) = args.get("top_k").and_then(Value::as_u64) {
                body["top_k"] = json!(top_k);
            }
            match ureq::post(&format!("{base}/v1/search")).send_json(body) {
                Ok(resp) => match resp.into_json::<Value>() {
                    Ok(v) => tool_text(&serde_json::to_string(&v).unwrap()),
                    Err(e) => tool_error(&format!("decode: {e}")),
                },
                Err(e) => tool_error(&format!("rest: {e}")),
            }
        }
        "open_citation" => {
            let token = args.get("citation").and_then(Value::as_str).unwrap_or("");
            let url = match std::env::var("GANESH_API_KEY") {
                Ok(key) if !key.is_empty() => format!("{base}/v1/citations/{token}?key={key}"),
                _ => format!("{base}/v1/citations/{token}"),
            };
            match ureq::get(&url).call() {
                Ok(resp) => match resp.into_json::<Value>() {
                    Ok(v) => tool_text(&serde_json::to_string(&v).unwrap()),
                    Err(e) => tool_error(&format!("decode: {e}")),
                },
                Err(ureq::Error::Status(404, _)) => tool_error("citation not found or not permitted"),
                Err(e) => tool_error(&format!("rest: {e}")),
            }
        }
        _ => tool_error(&format!("unknown tool: {name}")),
    }
}

fn tool_text(text: &str) -> Value {
    json!({ "content": [{ "type": "text", "text": text }] })
}

fn tool_error(msg: &str) -> Value {
    json!({ "content": [{ "type": "text", "text": msg }], "isError": true })
}

fn respond(out: &mut impl Write, id: Value, result: Value) {
    let msg = json!({ "jsonrpc": "2.0", "id": id, "result": result });
    writeln!(out, "{}", serde_json::to_string(&msg).unwrap()).unwrap();
    out.flush().unwrap();
}

fn respond_error(out: &mut impl Write, id: Value, code: i64, message: &str) {
    let msg = json!({
        "jsonrpc": "2.0", "id": id,
        "error": { "code": code, "message": message }
    });
    writeln!(out, "{}", serde_json::to_string(&msg).unwrap()).unwrap();
    out.flush().unwrap();
}

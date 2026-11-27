use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    response::Json,
    routing::{get, post},
    Router,
};
use ganesh_core::search::RetrievalConfig;
use ganesh_core::store::Store;
use ganesh_core::types::{ErrorBody, SearchRequest};
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::{Arc, Mutex};

/// Credential policy: the ONLY source of caller identity and permitted
/// audience. A request's self-asserted project or caller label can never
/// expand access; the key resolves to a fixed installation and project set.
#[derive(Debug, Clone, serde::Deserialize)]
struct Policy {
    #[serde(default)]
    keys: HashMap<String, KeyEntry>,
}

#[derive(Debug, Clone, serde::Deserialize)]
struct KeyEntry {
    #[serde(default)]
    installation: String,
    #[serde(default)]
    projects: Vec<String>,
}

impl Policy {
    fn resolve(&self, key: Option<&str>) -> Vec<String> {
        let mut projects = vec!["public".to_string()];
        if let Some(k) = key {
            if let Some(entry) = self.keys.get(k) {
                for p in &entry.projects {
                    if !projects.contains(p) {
                        projects.push(p.clone());
                    }
                }
            }
        }
        projects
    }
}

struct AppState {
    store: Mutex<Store>,
    config: RetrievalConfig,
    policy: Policy,
}

#[tokio::main]
async fn main() {
    ganesh_core::rerank::enforce_requirement("ganesh-server");
    ganesh_core::qa_verify::enforce_requirement("ganesh-server");
    ganesh_core::embed::enforce_requirement("ganesh-server");
    let mut db_path = None;
    let mut policy_path = None;
    let mut bind: SocketAddr = "127.0.0.1:0".parse().unwrap();
    let mut args = std::env::args().skip(1);
    while let Some(a) = args.next() {
        match a.as_str() {
            "--db" => db_path = args.next(),
            "--bind" => bind = args.next().expect("--bind value").parse().expect("bind addr"),
            "--policy" => policy_path = args.next(),
            other => {
                eprintln!("unknown argument: {other}");
                std::process::exit(2);
            }
        }
    }
    let db_path = db_path.expect("--db is required");
    let policy: Policy = match policy_path {
        Some(p) => serde_json::from_str(&std::fs::read_to_string(&p).expect("read policy")).expect("parse policy"),
        None => Policy { keys: HashMap::new() },
    };
    let store = Store::open(std::path::Path::new(&db_path)).expect("open store");
    let state = Arc::new(AppState {
        store: Mutex::new(store),
        config: RetrievalConfig::default(),
        policy,
    });

    let app = Router::new()
        .route("/v1/search", post(search_handler))
        .route("/v1/citations/:token", get(citation_handler))
        .route("/v1/health", get(|| async { "ok" }))
        .with_state(state);

    let listener = tokio::net::TcpListener::bind(bind).await.expect("bind");
    let addr = listener.local_addr().expect("local addr");
    // Machine-readable readiness line consumed by the bench harness.
    println!("listening {addr}");
    axum::serve(listener, app).await.expect("serve");
}

/// Environment scope mentioned in the query, for scoped supersession. Kept
/// deliberately minimal: exact lowercase word match on known scope terms.
fn query_scope_of(query: &str) -> String {
    let low = query.to_lowercase();
    for scope in ["staging", "production", "prod", "dev", "development"] {
        if low.split(|c: char| !c.is_alphanumeric()).any(|w| w == scope) {
            return if scope == "prod" { "production".to_string() } else { scope.to_string() };
        }
    }
    String::new()
}

async fn search_handler(
    State(state): State<Arc<AppState>>,
    Json(req): Json<SearchRequest>,
) -> Json<ganesh_core::types::SearchResponse> {
    // Access resolution is credential-only. `caller` remains an informational
    // label; `key` is resolved against the server policy. No request field can
    // widen the project set.
    let projects = state.policy.resolve(req.key.as_deref());
    let as_of = req.as_of.clone().unwrap_or_else(ganesh_core::types::now_iso);
    let top_k = req.top_k.unwrap_or(3);
    let query_scope = query_scope_of(&req.query);
    let query = req.query.clone();
    let as_of2 = as_of.clone();
    let cfg = state.config.clone();
    let state2 = Arc::clone(&state);
    let response = tokio::task::spawn_blocking(move || {
        let guard = state2.store.lock().unwrap();
        ganesh_core::search::search_full(&guard, &query, &projects, &as_of2, top_k, &query_scope, &cfg)
    })
    .await
    .unwrap_or_else(|_| ganesh_core::types::SearchResponse {
        status: "insufficient_context".to_string(),
        query: req.query.clone(),
        passages: vec![],
        confidence: "insufficient_context".to_string(),
        top_margin: None,
        effective_time: as_of.clone(),
    });
    Json(response)
}

#[derive(serde::Deserialize)]
struct CitationQuery {
    key: Option<String>,
}

async fn citation_handler(
    State(state): State<Arc<AppState>>,
    Path(token): Path<String>,
    Query(q): Query<CitationQuery>,
) -> Result<Json<ganesh_core::types::CitationOpen>, (StatusCode, Json<ErrorBody>)> {
    let projects = state.policy.resolve(q.key.as_deref());
    let caller = projects.join(",");
    let store = state.store.lock().unwrap();
    match store.open_citation(&token, &caller) {
        Ok(Some(open)) => Ok(Json(open)),
        Ok(None) => Err((
            StatusCode::NOT_FOUND,
            Json(ErrorBody {
                error: "citation not found or not permitted".to_string(),
            }),
        )),
        Err(e) => Err((
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ErrorBody { error: e.to_string() }),
        )),
    }
}

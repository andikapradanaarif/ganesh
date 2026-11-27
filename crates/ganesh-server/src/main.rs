use axum::{
    extract::{Path, State},
    http::StatusCode,
    response::Json,
    routing::{get, post},
    Router,
};
use ganesh_core::search::{search, RetrievalConfig};
use ganesh_core::store::Store;
use ganesh_core::types::{ErrorBody, SearchRequest};
use std::net::SocketAddr;
use std::sync::{Arc, Mutex};

struct AppState {
    store: Mutex<Store>,
    config: RetrievalConfig,
}

#[tokio::main]
async fn main() {
    ganesh_core::rerank::enforce_requirement("ganesh-server");
    ganesh_core::qa_verify::enforce_requirement("ganesh-server");
    let mut db_path = None;
    let mut bind: SocketAddr = "127.0.0.1:0".parse().unwrap();
    let mut args = std::env::args().skip(1);
    while let Some(a) = args.next() {
        match a.as_str() {
            "--db" => db_path = args.next(),
            "--bind" => bind = args.next().expect("--bind value").parse().expect("bind addr"),
            other => {
                eprintln!("unknown argument: {other}");
                std::process::exit(2);
            }
        }
    }
    let db_path = db_path.expect("--db is required");
    let store = Store::open(std::path::Path::new(&db_path)).expect("open store");
    let state = Arc::new(AppState {
        store: Mutex::new(store),
        config: RetrievalConfig::default(),
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

async fn search_handler(
    State(state): State<Arc<AppState>>,
    Json(req): Json<SearchRequest>,
) -> Json<ganesh_core::types::SearchResponse> {
    let caller = req.caller.clone().unwrap_or_else(|| "anonymous".to_string());
    let store = state.store.lock().unwrap();
    Json(search(&store, &req.query, &caller, &state.config))
}

async fn citation_handler(
    State(state): State<Arc<AppState>>,
    Path(token): Path<String>,
) -> Result<Json<ganesh_core::types::CitationOpen>, (StatusCode, Json<ErrorBody>)> {
    let store = state.store.lock().unwrap();
    match store.open_citation(&token, "anonymous") {
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

//! Owner Web API (M2 phase 2): inspection, correction with optimistic
//! concurrency, withdrawal, and source deletion — strictly owner-authenticated
//! under the credential policy. All mutations flow through the same
//! ganesh-core store service used by REST search and MCP.

use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    response::Html,
    Json,
};
use ganesh_core::store::{self, Store};
use ganesh_core::types::ManifestEntry;
use serde_json::{json, Value};
use std::sync::{Arc, Mutex};

use crate::AppState;

/// Owner authorization: the resolved key must carry `owner: true` in policy.
fn authorize(state: &AppState, key: Option<&str>) -> Result<Vec<String>, (StatusCode, Json<Value>)> {
    let Some(k) = key else {
        return Err((StatusCode::FORBIDDEN, Json(json!({"error": "owner credential required"}))));
    };
    let entry = state.policy.keys.get(k);
    match entry {
        Some(e) if e.owner => Ok(state.policy.resolve(Some(k))),
        _ => Err((StatusCode::FORBIDDEN, Json(json!({"error": "owner credential required"})))),
    }
}

#[derive(serde::Deserialize)]
pub struct KeyQuery {
    key: Option<String>,
}

#[derive(serde::Deserialize)]
pub struct ListQuery {
    key: Option<String>,
    project: Option<String>,
    scope: Option<String>,
    status: Option<String>,
    source_type: Option<String>,
}

pub async fn list_revisions_handler(
    State(state): State<Arc<AppState>>,
    Query(q): Query<ListQuery>,
) -> Result<Json<Value>, (StatusCode, Json<Value>)> {
    let projects = authorize(&state, q.key.as_deref())?;
    let guard = state.store.lock().unwrap();
    let rows = store::list_revisions(
        &guard,
        &projects,
        q.project.as_deref().unwrap_or(""),
        q.scope.as_deref().unwrap_or(""),
        q.status.as_deref().unwrap_or(""),
        q.source_type.as_deref().unwrap_or(""),
    );
    Ok(Json(json!({ "revisions": rows })))
}

pub async fn revision_detail_handler(
    State(state): State<Arc<AppState>>,
    Path(id): Path<String>,
    Query(q): Query<KeyQuery>,
) -> Result<Json<Value>, (StatusCode, Json<Value>)> {
    let projects = authorize(&state, q.key.as_deref())?;
    let guard = state.store.lock().unwrap();
    let rows = store::list_revisions(&guard, &projects, "", "", "", "");
    let Some(row) = rows.iter().find(|r| r.revision == id) else {
        return Err((StatusCode::NOT_FOUND, Json(json!({"error": "revision not found or not visible"}))));
    };
    let passages = store::revision_passages(&guard, &id);
    Ok(Json(json!({
        "revision": row,
        "passages": passages.iter().map(|(pid, section, text)| json!({
            "passage_id": pid, "section": section, "text": text,
        })).collect::<Vec<_>>(),
    })))
}

pub async fn revision_timeline_handler(
    State(state): State<Arc<AppState>>,
    Path(id): Path<String>,
    Query(q): Query<KeyQuery>,
) -> Result<Json<Value>, (StatusCode, Json<Value>)> {
    let projects = authorize(&state, q.key.as_deref())?;
    let guard = state.store.lock().unwrap();
    // Access rule: the timeline is visible only when the revision itself is
    // visible to this caller (or was before deletion — deleted revisions show
    // their purge events to the owner who is inspecting history).
    let rows = store::list_revisions(&guard, &projects, "", "", "", "");
    let visible = rows.iter().any(|r| r.revision == id);
    let timeline = store::revision_timeline(&guard, &id);
    if !visible && timeline.is_empty() {
        return Err((StatusCode::NOT_FOUND, Json(json!({"error": "revision not found or not visible"}))));
    }
    Ok(Json(json!({
        "revision": id,
        "visible_now": visible,
        "events": timeline.iter().map(|(ts, event, detail)| json!({
            "ts": ts, "event": event, "detail": detail,
        })).collect::<Vec<_>>(),
    })))
}

#[derive(serde::Deserialize)]
pub struct CorrectBody {
    key: Option<String>,
    /// Replacement claim text. When absent, only scope narrowing applies.
    new_text: Option<String>,
    /// Narrowed applicability scope (e.g. "staging"); empty keeps the original.
    narrow_scope: Option<String>,
    /// Optimistic concurrency precondition: the chain head the caller saw.
    expected_revision_id: Option<String>,
    /// Owner note recorded in the audit timeline.
    reason: Option<String>,
}

pub async fn correct_revision_handler(
    State(state): State<Arc<AppState>>,
    Path(id): Path<String>,
    Json(body): Json<CorrectBody>,
) -> Result<Json<Value>, (StatusCode, Json<Value>)> {
    let projects = authorize(&state, body.key.as_deref())?;
    let owner_key = body.key.clone().unwrap_or_default();
    let mut guard = state.store.lock().unwrap();

    let rows = store::list_revisions(&guard, &projects, "", "", "", "");
    let Some(orig) = rows.iter().find(|r| r.revision == id).cloned() else {
        return Err((StatusCode::NOT_FOUND, Json(json!({"error": "revision not found or not visible"}))));
    };

    // Optimistic concurrency: resolve the chain head; if it differs from what
    // the caller expects, the item changed while open — 409 with the newer payload.
    let succ = store::current_successor(&guard, &id);
    let head = if succ.is_empty() { id.clone() } else { succ };
    if let Some(expected) = &body.expected_revision_id {
        if *expected != head {
            let rows2 = store::list_revisions(&guard, &projects, "", "", "", "");
            let newer = rows2.iter().find(|r| r.revision == head);
            return Err((
                StatusCode::CONFLICT,
                Json(json!({
                    "error": "conflict: revision chain advanced while open",
                    "current_head": head,
                    "newer_revision": newer,
                })),
            ));
        }
    }

    // Compose the correction text: replacement, narrowed applicability note,
    // or verbatim carry-over when only scope narrows.
    let original_text: String = store::revision_passages(&guard, &head)
        .iter()
        .map(|(_, _, t)| t.clone())
        .collect::<Vec<_>>()
        .join("\n\n");
    let new_text = match (&body.new_text, &body.narrow_scope) {
        (Some(t), _) if !t.trim().is_empty() => t.clone(),
        (None, Some(scope)) if !scope.trim().is_empty() => original_text.clone(),
        _ => {
            return Err((StatusCode::BAD_REQUEST, Json(json!({"error": "new_text or narrow_scope required"}))));
        }
    };

    // Unique owner-correction revision id: head plus ordinal suffix.
    let ordinal = {
        let mut n = 1usize;
        loop {
            let cand = format!("{}-oc{}", head, n);
            let exists = store::revision_exists(&guard, &cand);
            if !exists {
                break n;
            }
            n += 1;
        }
    };
    let new_revision = format!("{}-oc{}", head, ordinal);
    let bytes = new_text.as_bytes().to_vec();
    let entry = ManifestEntry {
        article_id: orig.article_id.clone(),
        title: orig.title.clone(),
        pageid: 0,
        revision: new_revision.clone(),
        revision_url: String::new(),
        publisher: "owner".to_string(),
        url: String::new(),
        license: String::new(),
        license_url: String::new(),
        permission: String::new(),
        retrieved: String::new(),
        bytes: bytes.len() as u64,
        sha256: ganesh_core::citation::sha256_hex(&bytes),
        file: String::new(),
        family: String::new(),
        effective_from: None,
        effective_until: None,
        occurred_at: Some(ganesh_core::types::now_iso()),
        author: owner_key.clone(),
        source_type: orig.source_type.clone(),
        project: orig.project.clone(),
        scope: body.narrow_scope.clone().filter(|s| !s.trim().is_empty()).unwrap_or_else(|| orig.scope.clone()),
        authority: orig.authority.max(1),
        supersedes: Some(head.clone()),
        supersede_mode: if body.narrow_scope.as_deref().map(|s| !s.trim().is_empty()).unwrap_or(false) {
            "partial".to_string()
        } else {
            "full".to_string()
        },
        claim_key: orig.claim_key.clone(),
        audience: orig.audience.clone(),
        supports: vec![],
    };
    let mut store_ref: &mut Store = &mut guard;
    let outcome = store_ref.import_snapshot(&entry, &bytes);
    match outcome {
        Ok(_) => {
            store::insert_audit(
                &guard,
                "correct",
                &new_revision,
                &format!("owner {owner_key} corrected {head}: {}", body.reason.as_deref().unwrap_or("")),
            );
            guard.rebuild_index().ok();
            Ok(Json(json!({
                "status": "corrected",
                "revision": new_revision,
                "supersedes": head,
                "scope": entry.scope,
            })))
        }
        Err(e) => Err((StatusCode::INTERNAL_SERVER_ERROR, Json(json!({"error": format!("{e:?}")})))),
    }
}

#[derive(serde::Deserialize)]
pub struct WithdrawBody {
    key: Option<String>,
    reason: Option<String>,
}

pub async fn withdraw_revision_handler(
    State(state): State<Arc<AppState>>,
    Path(id): Path<String>,
    Json(body): Json<WithdrawBody>,
) -> Result<Json<Value>, (StatusCode, Json<Value>)> {
    let projects = authorize(&state, body.key.as_deref())?;
    let guard = state.store.lock().unwrap();
    let rows = store::list_revisions(&guard, &projects, "", "", "", "");
    if !rows.iter().any(|r| r.revision == id) {
        return Err((StatusCode::NOT_FOUND, Json(json!({"error": "revision not found or not visible"}))));
    }
    match store::withdraw_revision(&guard, &id) {
        Ok(()) => Ok(Json(json!({"status": "withdrawn", "revision": id}))),
        Err(e) => Err((StatusCode::INTERNAL_SERVER_ERROR, Json(json!({"error": format!("{e:?}")})))),
    }
}

pub async fn delete_source_handler(
    State(state): State<Arc<AppState>>,
    Path(id): Path<String>,
    Query(q): Query<KeyQuery>,
) -> Result<Json<Value>, (StatusCode, Json<Value>)> {
    let _projects = authorize(&state, q.key.as_deref())?;
    let guard = state.store.lock().unwrap();
    match store::delete_source(&guard, &id) {
        Ok((revs, passages)) => Ok(Json(json!({
            "status": "deleted",
            "source": id,
            "revisions_removed": revs,
            "passages_purged": passages,
        }))),
        Err(e) => Err((StatusCode::INTERNAL_SERVER_ERROR, Json(json!({"error": format!("{e:?}")})))),
    }
}

/// Embedded owner UI: zero external dependencies, vanilla JS against the API.
pub async fn ui_handler() -> Html<&'static str> {
    Html(include_str!("ui.html"))
}

use crate::citation;
use crate::segment;
use crate::types::{CitationOpen, ManifestEntry, SourceInfo};
use rusqlite::{params, Connection};
use std::fmt;

#[derive(Debug)]
pub enum StoreError {
    Sqlite(rusqlite::Error),
    /// Snapshot bytes do not match the digest recorded in the manifest.
    DigestMismatch { article_id: String, expected: String, actual: String },
    /// A revision identifier already exists with different bytes.
    ReusedRevisionConflict { revision: String },
    Utf8(std::str::Utf8Error),
}

impl fmt::Display for StoreError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            StoreError::Sqlite(e) => write!(f, "sqlite: {e}"),
            StoreError::DigestMismatch { article_id, expected, actual } => {
                write!(f, "digest mismatch for {article_id}: manifest {expected}, snapshot {actual}")
            }
            StoreError::ReusedRevisionConflict { revision } => {
                write!(f, "revision {revision} already imported with different bytes")
            }
            StoreError::Utf8(e) => write!(f, "utf8: {e}"),
        }
    }
}

impl std::error::Error for StoreError {}

impl From<rusqlite::Error> for StoreError {
    fn from(e: rusqlite::Error) -> Self {
        StoreError::Sqlite(e)
    }
}

#[derive(Debug, PartialEq, Eq)]
pub enum ImportOutcome {
    Imported { passages: usize },
    AlreadyPresent { passages: usize },
}

pub struct Store {
    pub conn: Connection,
}

impl Store {
    pub fn open(path: &std::path::Path) -> Result<Store, StoreError> {
        let conn = Connection::open(path)?;
        conn.execute_batch(
            "PRAGMA journal_mode = WAL;
             CREATE TABLE IF NOT EXISTS sources (
               article_id TEXT PRIMARY KEY,
               title TEXT NOT NULL,
               publisher TEXT NOT NULL,
               url TEXT NOT NULL,
               revision_url TEXT NOT NULL,
               license TEXT NOT NULL,
               retrieved TEXT NOT NULL,
               family TEXT NOT NULL
             );
             CREATE TABLE IF NOT EXISTS revisions (
               revision TEXT PRIMARY KEY,
               article_id TEXT NOT NULL REFERENCES sources(article_id),
               digest TEXT NOT NULL,
               bytes INTEGER NOT NULL,
               effective_from TEXT,
               effective_until TEXT,
               occurred_at TEXT,
               ingested_at TEXT NOT NULL DEFAULT '',
               author TEXT NOT NULL DEFAULT '',
               source_type TEXT NOT NULL DEFAULT 'wiki',
               project TEXT NOT NULL DEFAULT 'public',
               scope TEXT NOT NULL DEFAULT '',
               authority INTEGER NOT NULL DEFAULT 0,
               supersedes TEXT,
               supersede_mode TEXT NOT NULL DEFAULT '',
               claim_key TEXT NOT NULL DEFAULT '',
               audience TEXT NOT NULL DEFAULT ''
             );
             CREATE TABLE IF NOT EXISTS audit_events (
               id INTEGER PRIMARY KEY AUTOINCREMENT,
               ts TEXT NOT NULL,
               event TEXT NOT NULL,
               revision TEXT NOT NULL,
               detail TEXT NOT NULL DEFAULT ''
             );
             CREATE TABLE IF NOT EXISTS revocations (
               revision TEXT NOT NULL,
               project TEXT NOT NULL,
               PRIMARY KEY (revision, project)
             );
             CREATE TABLE IF NOT EXISTS supports (
               claim_revision TEXT NOT NULL,
               support_revision TEXT NOT NULL,
               PRIMARY KEY (claim_revision, support_revision)
             );
             CREATE TABLE IF NOT EXISTS embeddings (
               passage_id TEXT PRIMARY KEY REFERENCES passages(id),
               vector BLOB NOT NULL
             );
             CREATE TABLE IF NOT EXISTS passages (
               id TEXT PRIMARY KEY,
               revision TEXT NOT NULL REFERENCES revisions(revision),
               seq INTEGER NOT NULL,
               section TEXT NOT NULL,
               text TEXT NOT NULL
             );
             CREATE TABLE IF NOT EXISTS citations (
               token TEXT PRIMARY KEY,
               passage_id TEXT NOT NULL REFERENCES passages(id)
             );",
        )?;
        Ok(Store { conn })
    }

    /// Import one snapshot. Verifies the manifest digest, segments the bytes and
    /// records revision, passages and citations in one transaction. Reimporting the
    /// same revision and bytes is idempotent; a reused revision ID with different
    /// bytes is an error.
    pub fn import_snapshot(&mut self, entry: &ManifestEntry, bytes: &[u8]) -> Result<ImportOutcome, StoreError> {
        let actual = citation::sha256_hex(bytes);
        if actual != entry.sha256 {
            return Err(StoreError::DigestMismatch {
                article_id: entry.article_id.clone(),
                expected: entry.sha256.clone(),
                actual,
            });
        }
        if (bytes.len() as u64) != entry.bytes {
            return Err(StoreError::DigestMismatch {
                article_id: entry.article_id.clone(),
                expected: format!("{} bytes", entry.bytes),
                actual: format!("{} bytes", bytes.len()),
            });
        }

        let existing: Option<(String, i64)> = self
            .conn
            .query_row(
                "SELECT digest, bytes FROM revisions WHERE revision = ?1",
                params![entry.revision],
                |row| Ok((row.get(0)?, row.get(1)?)),
            )
            .ok();

        if let Some((digest, nbytes)) = existing {
            if digest != actual || nbytes != bytes.len() as i64 {
                return Err(StoreError::ReusedRevisionConflict {
                    revision: entry.revision.clone(),
                });
            }
            let count: i64 = self.conn.query_row(
                "SELECT COUNT(*) FROM passages WHERE revision = ?1",
                params![entry.revision],
                |row| row.get(0),
            )?;
            return Ok(ImportOutcome::AlreadyPresent {
                passages: count as usize,
            });
        }

        let text = std::str::from_utf8(bytes).map_err(StoreError::Utf8)?;
        let passages = segment::segment(text);
        let n = passages.len();

        let tx = self.conn.transaction()?;
        tx.execute(
            "INSERT INTO sources (article_id, title, publisher, url, revision_url, license, retrieved, family)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8)
             ON CONFLICT(article_id) DO NOTHING",
            params![
                entry.article_id, entry.title, entry.publisher, entry.url,
                entry.revision_url, entry.license, entry.retrieved, entry.family
            ],
        )?;
        let now = crate::types::now_iso();
        tx.execute(
            "INSERT INTO revisions (revision, article_id, digest, bytes, effective_from, effective_until, occurred_at, ingested_at, author, source_type, project, scope, authority, supersedes, supersede_mode, claim_key, audience)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15, ?16, ?17)",
            params![
                entry.revision, entry.article_id, actual, bytes.len() as i64,
                entry.effective_from, entry.effective_until, entry.occurred_at, now,
                entry.author, entry.source_type, entry.project, entry.scope, entry.authority,
                entry.supersedes, entry.supersede_mode, entry.claim_key, entry.audience,
            ],
        )?;
        tx.execute(
            "INSERT INTO audit_events (ts, event, revision, detail) VALUES (?1, 'import', ?2, ?3)",
            params![now, entry.revision, format!("article {} digest {}", entry.article_id, &actual[..12])],
        )?;
        for sup in &entry.supports {
            tx.execute(
                "INSERT OR IGNORE INTO supports (claim_revision, support_revision) VALUES (?1, ?2)",
                params![entry.revision, sup],
            )?;
        }
        for p in &passages {
            let id = format!("{}#p{}", entry.revision, p.seq);
            let section = if p.section.is_empty() {
                "(lead)".to_string()
            } else {
                p.section.join(" > ")
            };
            tx.execute(
                "INSERT INTO passages (id, revision, seq, section, text) VALUES (?1, ?2, ?3, ?4, ?5)",
                params![id, entry.revision, p.seq, section, p.text],
            )?;
            tx.execute(
                "INSERT INTO citations (token, passage_id) VALUES (?1, ?2)",
                params![citation::token_for(&id), id],
            )?;
        }
        // Dense embeddings for hybrid recall: embedded view carries the
        // metadata context header; stored verbatim text stays untouched.
        if crate::embed::available() {
            let rendered: Vec<String> = passages
                .iter()
                .map(|p| {
                    let section = if p.section.is_empty() { "(lead)".to_string() } else { p.section.join(" > ") };
                    crate::search::rendered_context_text(
                        entry.occurred_at.as_deref().unwrap_or(""),
                        &entry.source_type,
                        &entry.author,
                        &entry.title,
                        &section,
                        &p.text,
                    )
                })
                .collect();
            if let Some(vectors) = crate::embed::embed_batch(&rendered) {
                for (p, v) in passages.iter().zip(vectors.iter()) {
                    let id = format!("{}#p{}", entry.revision, p.seq);
                    let blob: Vec<u8> = v.iter().flat_map(|x| x.to_le_bytes()).collect();
                    tx.execute(
                        "INSERT OR REPLACE INTO embeddings (passage_id, vector) VALUES (?1, ?2)",
                        params![id, blob],
                    )?;
                }
            }
        }
        tx.commit()?;
        Ok(ImportOutcome::Imported { passages: n })
    }

    /// Rebuild the FTS5 search index from the accepted revisions. Indexed text is the
    /// passage's heading context plus its body. The rebuild is derived state only:
    /// passage IDs and citation tokens do not change.
    pub fn rebuild_index(&self) -> Result<usize, StoreError> {
        self.conn.execute_batch(
            "DROP TABLE IF EXISTS passages_fts;
             CREATE VIRTUAL TABLE passages_fts USING fts5(passage_id UNINDEXED, index_text);",
        )?;
        let mut stmt = self.conn.prepare(
            "SELECT p.id, p.text, p.section, s.title,
                    COALESCE(r.occurred_at,''), r.source_type, r.author
             FROM passages p JOIN revisions r ON p.revision = r.revision
             JOIN sources s ON r.article_id = s.article_id",
        )?;
        let rows = stmt.query_map([], |row| {
            Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?, row.get::<_, String>(2)?, row.get::<_, String>(3)?,
                row.get::<_, String>(4)?, row.get::<_, String>(5)?, row.get::<_, String>(6)?))
        })?;
        let mut count = 0usize;
        let mut embed_rows: Vec<(String, String)> = Vec::new();
        for row in rows {
            let (id, text, section, title, occurred, stype, author) = row?;
            let index_text = crate::search::stemmed_index_text_meta(&title, &section, &text, &occurred, &stype, &author);
            self.conn.execute(
                "INSERT INTO passages_fts (passage_id, index_text) VALUES (?1, ?2)",
                params![id, index_text],
            )?;
            embed_rows.push((id, crate::search::rendered_context_text(&occurred, &stype, &author, &title, &section, &text)));
            count += 1;
        }
        // Rebuild dense embeddings alongside FTS so the two stay in sync.
        if crate::embed::available() && !embed_rows.is_empty() {
            self.conn.execute("DELETE FROM embeddings", [])?;
            let rendered: Vec<String> = embed_rows.iter().map(|(_, r)| r.clone()).collect();
            if let Some(vectors) = crate::embed::embed_batch(&rendered) {
                for ((id, _), v) in embed_rows.iter().zip(vectors.iter()) {
                    let blob: Vec<u8> = v.iter().flat_map(|x| x.to_le_bytes()).collect();
                    self.conn.execute(
                        "INSERT OR REPLACE INTO embeddings (passage_id, vector) VALUES (?1, ?2)",
                        params![id, blob],
                    )?;
                }
            }
        }
        Ok(count)
    }

    pub fn passage_count(&self) -> Result<i64, StoreError> {
        Ok(self.conn.query_row("SELECT COUNT(*) FROM passages", [], |r| r.get(0))?)
    }

    /// Resolve a citation token. Re-checks the caller's permission at open time.
    /// M1's corpus is entirely scope `public`; the seam enforces that a passage is
    /// only returned when its scope is within the caller's permitted audience.
    pub fn open_citation(&self, token: &str, caller: &str) -> Result<Option<CitationOpen>, StoreError> {
        let row = self
            .conn
            .query_row(
                "SELECT p.id, p.revision, p.section, p.text, r.article_id,
                        s.title, s.publisher, s.url, s.revision_url, s.license
                 FROM citations c
                 JOIN passages p ON c.passage_id = p.id
                 JOIN revisions r ON p.revision = r.revision
                 JOIN sources s ON r.article_id = s.article_id
                 WHERE c.token = ?1",
                params![token],
                |row| {
                    Ok((
                        row.get::<_, String>(0)?, row.get::<_, String>(1)?, row.get::<_, String>(2)?,
                        row.get::<_, String>(3)?, row.get::<_, String>(4)?, row.get::<_, String>(5)?,
                        row.get::<_, String>(6)?, row.get::<_, String>(7)?, row.get::<_, String>(8)?,
                        row.get::<_, String>(9)?,
                    ))
                },
            )
            .ok();
        let Some((pid, revision, section, text, article_id, title, publisher, url, revision_url, license)) = row else {
            return Ok(None);
        };
        // Re-check permission at open time: the citation resolves only when the
        // caller can still see the underlying revision (visibility, revocation,
        // and — for multi-source claims — surviving support). `caller` here is
        // the caller's permitted-project list joined by commas; the server
        // resolves it strictly from credentials.
        let projects: Vec<String> = caller.split(',').map(|s| s.trim().to_string()).filter(|s| !s.is_empty()).collect();
        let projects = if projects.is_empty() { vec!["public".to_string()] } else { projects };
        let metas = all_revision_meta(self);
        if let Some(meta) = metas.iter().find(|m| m.revision == revision) {
            if !visible_to(self, meta, &projects) {
                return Ok(None);
            }
            let elig = eligible_revisions(self, &projects, "9999-12-31", "");
            if !elig.contains_key(&revision) {
                return Ok(None);
            }
        } else if !permitted(caller, "public") {
            return Ok(None);
        }
        Ok(Some(CitationOpen {
            citation: token.to_string(),
            passage_id: pid,
            revision,
            article_id: article_id.clone(),
            section,
            text,
            source: SourceInfo { article_id, title, publisher, url, revision_url, license },
        }))
    }
}

/// One access decision for passages, citations and derived output. M1 admits a
/// single public scope visible to every caller; the check is applied both when
/// returning passages and again when opening citations.
pub fn permitted(_caller: &str, scope: &str) -> bool {
    scope == "public"
}

// ============================ M2 lifecycle ============================

/// Caller context for one access decision. `projects` are resolved strictly
/// from credentials/policy — never from request parameters.
#[derive(Debug, Clone)]
pub struct AccessCtx {
    /// Permitted project scopes for this caller (always includes "public").
    pub projects: Vec<String>,
}

impl AccessCtx {
    pub fn public_only() -> Self {
        AccessCtx { projects: vec!["public".to_string()] }
    }
}

/// Revision metadata needed for eligibility decisions.
#[derive(Debug, Clone)]
pub struct RevisionMeta {
    pub revision: String,
    pub article_id: String,
    pub effective_from: Option<String>,
    pub effective_until: Option<String>,
    pub occurred_at: Option<String>,
    pub author: String,
    pub source_type: String,
    pub project: String,
    pub scope: String,
    pub authority: i64,
    pub supersedes: Option<String>,
    pub supersede_mode: String,
    pub claim_key: String,
    pub audience: String,
}

fn csv_contains(csv: &str, item: &str) -> bool {
    csv.split(',').map(|s| s.trim()).any(|s| s == item)
}

/// Visibility of one revision to a caller with the given permitted projects.
/// A revision is visible when any caller project grants access and no
/// revocation row blocks that granting project.
pub fn visible_to(store: &Store, meta: &RevisionMeta, projects: &[String]) -> bool {
    for p in projects {
        let grants = meta.project == "public"
            || meta.project == *p
            || csv_contains(&meta.audience, p)
            || csv_contains(&meta.audience, "public");
        if !grants {
            continue;
        }
        let revoked: bool = store
            .conn
            .query_row(
                "SELECT 1 FROM revocations WHERE revision = ?1 AND project = ?2",
                params![meta.revision, p],
                |_| Ok(()),
            )
            .is_ok();
        if !revoked {
            return true;
        }
    }
    false
}

/// Every revision row in the store with its lifecycle metadata.
pub fn all_revision_meta(store: &Store) -> Vec<RevisionMeta> {
    let mut stmt = match store.conn.prepare(
        "SELECT revision, article_id, effective_from, effective_until, occurred_at,
                author, source_type, project, scope, authority, supersedes, supersede_mode, claim_key, audience
         FROM revisions",
    ) {
        Ok(s) => s,
        Err(_) => return vec![],
    };
    stmt.query_map([], |row| {
        Ok(RevisionMeta {
            revision: row.get(0)?,
            article_id: row.get(1)?,
            effective_from: row.get(2)?,
            effective_until: row.get(3)?,
            occurred_at: row.get(4)?,
            author: row.get::<_, String>(5)?,
            source_type: row.get::<_, String>(6)?,
            project: row.get::<_, String>(7)?,
            scope: row.get::<_, String>(8)?,
            authority: row.get(9)?,
            supersedes: row.get(10)?,
            supersede_mode: row.get::<_, String>(11)?,
            claim_key: row.get::<_, String>(12)?,
            audience: row.get::<_, String>(13)?,
        })
    })
    .map(|r| r.filter_map(|x| x.ok()).collect())
    .unwrap_or_default()
}

fn temporal_ok(meta: &RevisionMeta, as_of: &str) -> bool {
    if let Some(f) = &meta.effective_from {
        if !f.is_empty() && f.as_str() > as_of {
            return false;
        }
    }
    if let Some(u) = &meta.effective_until {
        if !u.is_empty() && u.as_str() < as_of {
            return false;
        }
    }
    true
}

/// Whether revision `s` actively supersedes revision `r` for a query about
/// scope `query_scope` (empty = unscoped/broad query). A scoped correction
/// retires its predecessor only inside its own scope for scoped reads; a
/// global correction (empty scope) retires it everywhere. Phase-1 semantics:
/// broad unscoped reads apply only global supersessions.
fn supersedes_in_scope(s: &RevisionMeta, query_scope: &str) -> bool {
    // Full supersession (explicit mode or a global scope-less correction)
    // retires the predecessor in every scope. A partial scoped correction
    // retires it only inside the correction's own scope for scoped reads,
    // and never for broad unscoped reads (phase-1 semantics).
    if s.supersede_mode == "full" || s.scope.is_empty() {
        return true;
    }
    !query_scope.is_empty() && s.scope == query_scope
}

/// Compute the eligible revision set for one read: visible to the caller,
/// inside its effective window at `as_of`, not superseded by an eligible
/// successor in scope, and — for multi-source claims — still backed by at
/// least one visible support. Computed at read time, so import order and
/// late arrivals can never revive a superseded revision.
pub fn eligible_revisions(store: &Store, projects: &[String], as_of: &str, query_scope: &str) -> std::collections::HashMap<String, RevisionMeta> {
    let all = all_revision_meta(store);
    let mut base: std::collections::HashMap<String, RevisionMeta> = std::collections::HashMap::new();
    for m in &all {
        if !visible_to(store, m, projects) {
            continue;
        }
        if !temporal_ok(m, as_of) {
            continue;
        }
        base.insert(m.revision.clone(), m.clone());
    }
    // Supersession is computed over the scope-UNFILTERED base: a narrowing
    // correction (full supersession with narrowed scope) retracts its
    // predecessor everywhere even though the successor itself serves only
    // its own scope. Scope visibility filtering happens after supersession.
    let mut scope_filtered = base.clone();
    if !query_scope.is_empty() {
        scope_filtered.retain(|_, m| m.scope.is_empty() || m.scope == query_scope);
    }
    let mut base = scope_filtered;
    // shadow for the passes below: supersession looks at the unfiltered set
    let unfiltered = {
        let mut u = std::collections::HashMap::new();
        for m in &all {
            if visible_to(store, m, projects) && temporal_ok(m, as_of) {
                u.insert(m.revision.clone(), m.clone());
            }
        }
        u
    };
    // Supersession pass: drop R when an eligible S supersedes it in scope.
    // Successors are drawn from the unfiltered visible+temporal set so a
    // narrowing correction retracts its predecessor outside its own scope.
    let mut superseded: std::collections::HashSet<String> = std::collections::HashSet::new();
    for r in base.values() {
        for s in unfiltered.values() {
            if s.revision != r.revision {
                if let Some(target) = &s.supersedes {
                    if *target == r.revision && supersedes_in_scope(s, query_scope) {
                        superseded.insert(r.revision.clone());
                    }
                }
            }
        }
    }
    for rev in &superseded {
        base.remove(rev);
    }
    // Multi-source support pass: a claim revision with support links stays
    // only while at least one support is itself visible and temporally
    // eligible to this caller.
    let mut supported_out: Vec<String> = Vec::new();
    for r in base.values() {
        let mut stmt = match store.conn.prepare(
            "SELECT support_revision FROM supports WHERE claim_revision = ?1",
        ) {
            Ok(s) => s,
            Err(_) => continue,
        };
        let supports: Vec<String> = stmt
            .query_map(params![r.revision], |row| row.get(0))
            .map(|rows| rows.filter_map(|x| x.ok()).collect())
            .unwrap_or_default();
        if supports.is_empty() {
            continue;
        }
        let any_valid = supports.iter().any(|sup| {
            all.iter().any(|m| {
                m.revision == *sup && visible_to(store, m, projects) && temporal_ok(m, as_of)
            })
        });
        if !any_valid {
            supported_out.push(r.revision.clone());
        }
    }
    for rev in &supported_out {
        base.remove(rev);
    }
    base
}

/// Revoke one revision's visibility for one project. Persisted in SQLite, so
/// it survives restarts and is re-checked on every read and citation open.
pub fn revoke(store: &Store, revision: &str, project: &str) -> Result<(), StoreError> {
    store.conn.execute(
        "INSERT OR IGNORE INTO revocations (revision, project) VALUES (?1, ?2)",
        params![revision, project],
    )?;
    store.conn.execute(
        "INSERT INTO audit_events (ts, event, revision, detail) VALUES (?1, 'revoke', ?2, ?3)",
        params![crate::types::now_iso(), revision, format!("project {project}")],
    )?;
    Ok(())
}

/// Delete one revision and its derived rows (passages, citations, embeddings)
/// under the retention policy. Returns the number of passages removed.
pub fn delete_revision(store: &Store, revision: &str) -> Result<usize, StoreError> {
    let pids: Vec<String> = {
        let mut stmt = store.conn.prepare("SELECT id FROM passages WHERE revision = ?1")?;
        stmt.query_map(params![revision], |row| row.get(0))
            .map(|r| r.filter_map(|x| x.ok()).collect())
            .unwrap_or_default()
    };
    let n = pids.len();
    for pid in &pids {
        store.conn.execute("DELETE FROM citations WHERE passage_id = ?1", params![pid])?;
        store.conn.execute("DELETE FROM embeddings WHERE passage_id = ?1", params![pid])?;
        store.conn.execute("DELETE FROM passages_fts WHERE passage_id = ?1", params![pid])?;
    }
    store.conn.execute("DELETE FROM passages WHERE revision = ?1", params![revision])?;
    store.conn.execute("DELETE FROM supports WHERE claim_revision = ?1 OR support_revision = ?1", params![revision])?;
    store.conn.execute("DELETE FROM revisions WHERE revision = ?1", params![revision])?;
    store.conn.execute(
        "INSERT INTO audit_events (ts, event, revision, detail) VALUES (?1, 'delete', ?2, ?3)",
        params![crate::types::now_iso(), revision, format!("{n} passages removed")],
    )?;
    Ok(n)
}

/// The revision visibility scope label used by citations. Passages inherit
/// the project of their revision.
pub fn revision_project(store: &Store, revision: &str) -> String {
    store
        .conn
        .query_row(
            "SELECT project FROM revisions WHERE revision = ?1",
            params![revision],
            |row| row.get(0),
        )
        .unwrap_or_else(|_| "public".to_string())
}

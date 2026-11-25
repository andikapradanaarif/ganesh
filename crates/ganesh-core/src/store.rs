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
               bytes INTEGER NOT NULL
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
        tx.execute(
            "INSERT INTO revisions (revision, article_id, digest, bytes) VALUES (?1, ?2, ?3, ?4)",
            params![entry.revision, entry.article_id, actual, bytes.len() as i64],
        )?;
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
            "SELECT p.id, p.text, p.section, s.title
             FROM passages p JOIN revisions r ON p.revision = r.revision
             JOIN sources s ON r.article_id = s.article_id",
        )?;
        let rows = stmt.query_map([], |row| {
            Ok((row.get::<_, String>(0)?, row.get::<_, String>(1)?, row.get::<_, String>(2)?, row.get::<_, String>(3)?))
        })?;
        let mut count = 0usize;
        for row in rows {
            let (id, text, section, title) = row?;
            let index_text = crate::search::stemmed_index_text(&title, &section, &text);
            self.conn.execute(
                "INSERT INTO passages_fts (passage_id, index_text) VALUES (?1, ?2)",
                params![id, index_text],
            )?;
            count += 1;
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
        if !permitted(caller, "public") {
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

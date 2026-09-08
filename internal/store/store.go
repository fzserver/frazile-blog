// Package store is the blog's SQLite persistence: users, sessions, e-mail
// verification codes, posts, tags, comments, likes, uploaded media and site
// settings all live in one file. Traffic statistics are deliberately kept in a
// separate database (internal/stats) so a burst of visits never contends with
// content writes.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrTaken          = errors.New("already taken")
	ErrBadCredentials = errors.New("bad credentials")
	ErrNoSession      = errors.New("no such session")
)

type Store struct {
	db *sql.DB
	// Iters is the PBKDF2 work factor for new passwords; tests lower it.
	Iters int
}

const defaultIters = 210_000

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	// A single writer keeps SQLite happy; reads still run concurrently in WAL.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema: %w", err)
	}
	for _, mig := range migrations {
		if _, err := db.Exec(mig); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			db.Close()
			return nil, fmt.Errorf("migration: %w", err)
		}
	}
	return &Store{db: db, Iters: defaultIters}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// DB exposes the handle for the few places (tests, tools) that need raw SQL.
func (s *Store) DB() *sql.DB { return s.db }

const schema = `
CREATE TABLE IF NOT EXISTS users(
	id           INTEGER PRIMARY KEY,
	username     TEXT NOT NULL COLLATE NOCASE UNIQUE,
	email        TEXT NOT NULL COLLATE NOCASE UNIQUE,
	display_name TEXT NOT NULL DEFAULT '',
	bio          TEXT NOT NULL DEFAULT '',
	website      TEXT NOT NULL DEFAULT '',
	avatar       TEXT NOT NULL DEFAULT '',
	role         TEXT NOT NULL DEFAULT 'reader',
	verified     INTEGER NOT NULL DEFAULT 0,
	banned       INTEGER NOT NULL DEFAULT 0,
	salt         BLOB NOT NULL,
	hash         BLOB NOT NULL,
	iters        INTEGER NOT NULL,
	created      INTEGER NOT NULL,
	last_seen    INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS sessions(
	token_hash BLOB PRIMARY KEY,
	user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created    INTEGER NOT NULL,
	expires    INTEGER NOT NULL,
	ua         TEXT NOT NULL DEFAULT '',
	ip         TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS sessions_user ON sessions(user_id);
CREATE TABLE IF NOT EXISTS email_codes(
	email    TEXT NOT NULL COLLATE NOCASE,
	purpose  TEXT NOT NULL,
	hash     BLOB NOT NULL,
	expires  INTEGER NOT NULL,
	attempts INTEGER NOT NULL DEFAULT 0,
	sent     INTEGER NOT NULL,
	PRIMARY KEY(email, purpose)
);
CREATE TABLE IF NOT EXISTS posts(
	id               INTEGER PRIMARY KEY,
	slug             TEXT NOT NULL UNIQUE,
	title            TEXT NOT NULL,
	summary          TEXT NOT NULL DEFAULT '',
	body_md          TEXT NOT NULL DEFAULT '',
	body_html        TEXT NOT NULL DEFAULT '',
	cover            TEXT NOT NULL DEFAULT '',
	author_id        INTEGER NOT NULL REFERENCES users(id),
	status           TEXT NOT NULL DEFAULT 'draft',
	published_at     INTEGER NOT NULL DEFAULT 0,
	created          INTEGER NOT NULL,
	updated          INTEGER NOT NULL,
	views            INTEGER NOT NULL DEFAULT 0,
	likes            INTEGER NOT NULL DEFAULT 0,
	comments_enabled INTEGER NOT NULL DEFAULT 1,
	pinned           INTEGER NOT NULL DEFAULT 0,
	reading_min      INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX IF NOT EXISTS posts_pub ON posts(status, published_at);
CREATE INDEX IF NOT EXISTS posts_author ON posts(author_id);
CREATE TABLE IF NOT EXISTS tags(
	id   INTEGER PRIMARY KEY,
	name TEXT NOT NULL COLLATE NOCASE UNIQUE,
	slug TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS post_tags(
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	tag_id  INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
	PRIMARY KEY(post_id, tag_id)
);
CREATE INDEX IF NOT EXISTS post_tags_tag ON post_tags(tag_id);
CREATE TABLE IF NOT EXISTS comments(
	id        INTEGER PRIMARY KEY,
	post_id   INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	parent_id INTEGER NOT NULL DEFAULT 0,
	body      TEXT NOT NULL,
	status    TEXT NOT NULL DEFAULT 'visible',
	created   INTEGER NOT NULL,
	edited    INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS comments_post ON comments(post_id, created);
CREATE INDEX IF NOT EXISTS comments_user ON comments(user_id);
CREATE TABLE IF NOT EXISTS post_likes(
	post_id INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
	user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	created INTEGER NOT NULL,
	PRIMARY KEY(post_id, user_id)
);
CREATE TABLE IF NOT EXISTS media(
	id            INTEGER PRIMARY KEY,
	name          TEXT NOT NULL UNIQUE,
	original_name TEXT NOT NULL DEFAULT '',
	mime          TEXT NOT NULL,
	kind          TEXT NOT NULL,
	size          INTEGER NOT NULL,
	width         INTEGER NOT NULL DEFAULT 0,
	height        INTEGER NOT NULL DEFAULT 0,
	uploader_id   INTEGER NOT NULL DEFAULT 0,
	created       INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS settings(
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);
CREATE VIRTUAL TABLE IF NOT EXISTS posts_fts USING fts5(
	title, summary, body_md, content='posts', content_rowid='id', tokenize='porter unicode61'
);
CREATE TRIGGER IF NOT EXISTS posts_ai AFTER INSERT ON posts BEGIN
	INSERT INTO posts_fts(rowid, title, summary, body_md) VALUES (new.id, new.title, new.summary, new.body_md);
END;
CREATE TRIGGER IF NOT EXISTS posts_ad AFTER DELETE ON posts BEGIN
	INSERT INTO posts_fts(posts_fts, rowid, title, summary, body_md) VALUES ('delete', old.id, old.title, old.summary, old.body_md);
END;
CREATE TRIGGER IF NOT EXISTS posts_au AFTER UPDATE ON posts BEGIN
	INSERT INTO posts_fts(posts_fts, rowid, title, summary, body_md) VALUES ('delete', old.id, old.title, old.summary, old.body_md);
	INSERT INTO posts_fts(rowid, title, summary, body_md) VALUES (new.id, new.title, new.summary, new.body_md);
END;
`

// migrations run every start; "duplicate column" is the already-applied case.
var migrations = []string{}

func isUniqueErr(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "2067"))
}

func now() int64 { return time.Now().Unix() }

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Tx runs fn inside a transaction.
func (s *Store) Tx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

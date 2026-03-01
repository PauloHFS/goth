-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS posts (
    id INTEGER PRIMARY KEY,
    tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    content TEXT NOT NULL
);

CREATE VIRTUAL TABLE IF NOT EXISTS posts_idx USING fts5(title, content, content='posts', content_rowid='id');

-- Triggers
DROP TRIGGER IF EXISTS posts_ai;
-- +goose StatementBegin
CREATE TRIGGER posts_ai AFTER INSERT ON posts
BEGIN
  INSERT OR IGNORE INTO posts_idx(rowid, title, content)
  VALUES (new.id, COALESCE(new.title, ''), COALESCE(new.content, ''));
END;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS posts_ad;
-- +goose StatementBegin
CREATE TRIGGER posts_ad AFTER DELETE ON posts
BEGIN
  INSERT INTO posts_idx(posts_idx, rowid, title, content)
  VALUES('delete', old.id, COALESCE(old.title, ''), COALESCE(old.content, ''));
END;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS posts_au;
-- +goose StatementBegin
CREATE TRIGGER posts_au AFTER UPDATE ON posts
BEGIN
  INSERT INTO posts_idx(posts_idx, rowid, title, content)
  VALUES('delete', old.id, COALESCE(old.title, ''), COALESCE(old.content, ''));
  INSERT OR IGNORE INTO posts_idx(rowid, title, content)
  VALUES(new.id, COALESCE(new.title, ''), COALESCE(new.content, ''));
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS posts_au;
DROP TRIGGER IF EXISTS posts_ad;
DROP TRIGGER IF EXISTS posts_ai;
DROP TABLE IF EXISTS posts_idx;
DROP TABLE IF EXISTS posts;

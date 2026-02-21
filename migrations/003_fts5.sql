-- ===========================================
-- 003: FTS5 Full-Text Search
-- ===========================================
-- Tabelas FTS5 com External Content
-- Dados são armazenados na tabela original, FTS5 apenas indexa

-- Users FTS5 (External Content)
CREATE VIRTUAL TABLE IF NOT EXISTS users_fts USING fts5(
    email,
    content='users',
    content_rowid='id'
);

-- Tenants FTS5 (sem External Content - id é TEXT)
CREATE VIRTUAL TABLE IF NOT EXISTS tenants_fts USING fts5(
    name,
    content=''
);

-- Activity Logs FTS5 (External Content)
CREATE VIRTUAL TABLE IF NOT EXISTS activity_logs_fts USING fts5(
    action, entity_type,
    content='activity_logs',
    content_rowid='id'
);

-- ===========================================
-- FTS5 Triggers (Auto-sync)
-- ===========================================

-- Users FTS Triggers
CREATE TRIGGER IF NOT EXISTS users_ai AFTER INSERT ON users BEGIN
    INSERT INTO users_fts(rowid, email) VALUES (new.id, new.email);
END;

CREATE TRIGGER IF NOT EXISTS users_ad AFTER DELETE ON users BEGIN
    INSERT INTO users_fts(users_fts, rowid, email) 
    VALUES('delete', old.id, old.email);
END;

CREATE TRIGGER IF NOT EXISTS users_au AFTER UPDATE ON users BEGIN
    INSERT INTO users_fts(users_fts, rowid, email) 
    VALUES('delete', old.id, old.email);
    INSERT INTO users_fts(rowid, email) VALUES (new.id, new.email);
END;

-- Tenants FTS Triggers
-- Nota: tenants.id é TEXT, então não podemos usar External Content
-- O trigger insere diretamente na FTS5 com rowid automático
CREATE TRIGGER IF NOT EXISTS tenants_ai AFTER INSERT ON tenants BEGIN
    INSERT INTO tenants_fts(name) VALUES (new.name);
END;

CREATE TRIGGER IF NOT EXISTS tenants_ad AFTER DELETE ON tenants BEGIN
    INSERT INTO tenants_fts(tenants_fts, name) 
    VALUES('delete', old.name);
END;

CREATE TRIGGER IF NOT EXISTS tenants_au AFTER UPDATE ON tenants BEGIN
    INSERT INTO tenants_fts(tenants_fts, name) 
    VALUES('delete', old.name);
    INSERT INTO tenants_fts(name) VALUES (new.name);
END;

-- Activity Logs FTS Triggers
CREATE TRIGGER IF NOT EXISTS activity_logs_ai AFTER INSERT ON activity_logs BEGIN
    INSERT INTO activity_logs_fts(rowid, action, entity_type) 
    VALUES (new.id, new.action, new.entity_type);
END;

CREATE TRIGGER IF NOT EXISTS activity_logs_ad AFTER DELETE ON activity_logs BEGIN
    INSERT INTO activity_logs_fts(activity_logs_fts, rowid, action, entity_type) 
    VALUES('delete', old.id, old.action, old.entity_type);
END;

CREATE TRIGGER IF NOT EXISTS activity_logs_au AFTER UPDATE ON activity_logs BEGIN
    INSERT INTO activity_logs_fts(activity_logs_fts, rowid, action, entity_type) 
    VALUES('delete', old.id, old.action, old.entity_type);
    INSERT INTO activity_logs_fts(rowid, action, entity_type) 
    VALUES (new.id, new.action, new.entity_type);
END;

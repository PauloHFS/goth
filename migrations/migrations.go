package migrations

import "embed"

// FS expõe o diretório de migrations para uso com goose.
// Inclui arquivos .sql no formato goose (-- +goose Up/Down).
//
//go:embed *.sql
var FS embed.FS

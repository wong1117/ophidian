package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ophidian/ophidian/internal/domain/common"
)

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func marshalJSON(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

func unmarshalJSON(data []byte, v interface{}) error {
	if len(data) == 0 {
		return fmt.Errorf("empty json data")
	}
	return json.Unmarshal(data, v)
}

func MarshalJSON(v interface{}) []byte  { return marshalJSON(v) }
func UnmarshalJSON(d []byte, v interface{}) error { return unmarshalJSON(d, v) }

func wrapNotFound(err error, entity, id string) error {
	if err == pgx.ErrNoRows {
		return fmt.Errorf("%w: %s with id %s not found", common.ErrMissionNotFound, entity, id)
	}
	return err
}

func wrapSaveError(err error, entity string) error {
	if err != nil {
		return fmt.Errorf("save %s: %w", entity, err)
	}
	return nil
}

func wrapUpdateError(err error, entity string) error {
	if err != nil {
		return fmt.Errorf("update %s: %w", entity, err)
	}
	return nil
}

func wrapDeleteError(err error, entity string) error {
	if err != nil {
		return fmt.Errorf("delete %s: %w", entity, err)
	}
	return nil
}

type RepoDeps struct {
	Query    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow func(ctx context.Context, sql string, args ...any) pgx.Row
	Exec     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func repoDepsFromPool(pool *pgxpool.Pool) RepoDeps {
	return RepoDeps{
		Query:    pool.Query,
		QueryRow: pool.QueryRow,
		Exec:     pool.Exec,
	}
}

func RepoDepsFromPool(pool *pgxpool.Pool) RepoDeps { return repoDepsFromPool(pool) }

package traces

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/proto"

	"github.com/kanengo/akasar/runtime/protos"

	"github.com/kanengo/akasar/runtime/retry"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// DB 一个储存 traces 在本地文件的 trace 数据库
type DB struct {
	fName string
	db    *sql.DB
}

func OpenDB(ctx context.Context, fName string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(fName), 0700); err != nil {
		return nil, err
	}

	const params = "?_locking_mode=NORMAL&_busy_timeout=10000"
	db, err := sql.Open("sqlite", fName+params)
	if err != nil {
		return nil, fmt.Errorf("open db %q failed: %w", fName, err)
	}
	db.SetMaxOpenConns(1)

	t := &DB{
		fName: fName,
		db:    db,
	}

	const initDB = `
-- Disable foreign key checking as spans may get inserted into encoded_spans
-- before the corresponding traces are inserted into traces.
PRAGMA foreign_keys=OFF;

-- Queryable trace data.
CREATE TABLE IF NOT EXISTS traces (
	trace_id TEXT NOT NULL,
	app TEXT NOT NULL,
	version TEXT NOT NULL,
	name TEXT,
	start_time_unix_us INTEGER,
	end_time_unix_us INTEGER,
	status TEXT,
	PRIMARY KEY(trace_id)
);

-- Encoded spans.
CREATE TABLE IF NOT EXISTS encoded_spans (
	trace_id TEXT NOT NULL,
	start_time_unix_us INTEGER,
	data TEXT,
	FOREIGN KEY (trace_id) REFERENCES traces (trace_id)
);

-- Garbage-collect traces older than 30 days.
CREATE TRIGGER IF NOT EXISTS expire_traces AFTER INSERT ON traces
BEGIN
	DELETE FROM traces
	WHERE start_time_unix_us < (1000000 * unixepoch('now', '-30 days'));
END;

-- Garbage-collect spans older than 30 days.
CREATE TRIGGER IF NOT EXISTS expire_spans AFTER INSERT ON encoded_spans
BEGIN
	DELETE FROM encoded_spans
	WHERE start_time_unix_us < (1000000 * unixepoch('now', '-30 days'));
END;
`

	if _, err := t.execDB(ctx, initDB); err != nil {
		return nil, fmt.Errorf("open trace DB %s: %w", fName, err)
	}

	return t, nil
}

func (t *DB) execDB(ctx context.Context, query string, args ...any) (sql.Result, error) {
	for r := retry.Begin(); r.Continue(ctx); {
		res, err := t.db.ExecContext(ctx, query, args...)
		if isLocked(err) {
			continue
		}
		return res, err
	}
	return nil, ctx.Err()
}

// isLocked returns whether the error is a "database is locked" error.
func isLocked(err error) bool {
	sqlError := &sqlite.Error{}
	ok := errors.As(err, &sqlError)

	return ok && (sqlError.Code() == sqlite3.SQLITE_BUSY || sqlError.Code() == sqlite3.SQLITE_LOCKED)
}

func (t *DB) Close() error {
	return t.db.Close()
}

func isRootSpan(span *protos.Span) bool {
	var nilSpanId [8]byte
	return bytes.Equal(span.ParentSpanId, nilSpanId[:])
}

func (t *DB) storeTrace(ctx context.Context, tx *sql.Tx, app, version string, root *protos.Span) error {
	const traceStmt = `INSERT INTO traces VALUES (?,?,?,?,?,?,?)`
	_, err := tx.ExecContext(ctx, traceStmt, hex.EncodeToString(root.TraceId), app, version, root.Name, root.StartMicros,
		root.EndMicros, spanStatus(root))
	return err
}

func (t *DB) storeSpan(ctx context.Context, tx *sql.Tx, span *protos.Span) error {
	encoded, err := proto.Marshal(span)
	if err != nil {
		return err
	}
	const stmt = `INSERT INTO encoded_spans VALUES (?,?,?)`
	_, err = tx.ExecContext(ctx, stmt, hex.EncodeToString(span.TraceId), span.StartMicros, encoded)

	return err
}

func (t *DB) Store(ctx context.Context, app, version string, spans *protos.TraceSpans) error { // NOTE: we insert all rows transactionally, as it is significantly faster
	// than inserting one row at a time [1].
	//
	// [1]: https://stackoverflow.com/questions/1711631/improve-insert-per-second-performance-of-sqlite

	tx, err := t.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelLinearizable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var errs []error
	for _, span := range spans.Span {
		if isRootSpan(span) {
			if err := t.storeTrace(ctx, tx, app, version, span); err != nil {
				errs = append(errs, err)
			}
		}
		if err := t.storeSpan(ctx, tx, span); err != nil {
			errs = append(errs, err)
		}
	}

	if errs != nil {
		return errors.Join(errs...)
	}

	return tx.Commit()
}

// spanStatus returns the span status string. It returns "" if the status is OK.
func spanStatus(span *protos.Span) string {
	// Look for an error in the span status.
	if span.Status != nil && span.Status.Code == protos.Span_Status_ERROR {
		if span.Status.Error != "" {
			return span.Status.Error
		} else {
			return "unknown error"
		}
	}

	// Look for an HTTP error in the span attributes.
	for _, attr := range span.Attributes {
		if attr.Key != "http.status_code" {
			continue
		}
		if attr.Value.Type != protos.Span_Attribute_Value_INT64 {
			continue
		}
		val, ok := attr.Value.Value.(*protos.Span_Attribute_Value_Num)
		if !ok {
			continue
		}
		if val.Num >= 400 && val.Num < 600 {
			return http.StatusText(int(val.Num))
		}
	}

	// No errors found.
	return ""
}

package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"modernc.org/sqlite"
)

var (
	registerOnce sync.Once
	schemaRe     = regexp.MustCompile(`\b(identity|social|chat|realtime|calling|crypto|community|business|developer|creator|commerce|ads|safety|ops)\.([a-zA-Z_][a-zA-Z0-9_]*)\b`)
	paramRe      = regexp.MustCompile(`\$(\d+)`)
	ilikeRe      = regexp.MustCompile(`(?i)\bILIKE\b`)
	forUpdateRe  = regexp.MustCompile(`(?i)\s+FOR\s+UPDATE\b`)
	createSchRe  = regexp.MustCompile(`(?i)^\s*CREATE\s+SCHEMA\s+IF\s+NOT\s+EXISTS\s+[a-zA-Z0-9_]+\s*;?\s*$`)
	identityPKRe = regexp.MustCompile(`(?i)\bBIGINT\s+GENERATED\s+ALWAYS\s+AS\s+IDENTITY\s+PRIMARY\s+KEY\b`)
	castJsonbRe  = regexp.MustCompile(`(?i)::(jsonb|int)\b`)
	textArrRe    = regexp.MustCompile(`(?i)\bTEXT\[\]\s+NOT\s+NULL\s+DEFAULT\s+'\{\}'`)
	nowFuncRe    = regexp.MustCompile(`(?i)\bnow\(\)`)
	uuidFuncRe   = regexp.MustCompile(`(?i)\bgen_random_uuid\(\)`)
	greatestRe   = regexp.MustCompile(`(?i)\bGREATEST\s*\(`)
	leastRe      = regexp.MustCompile(`(?i)\bLEAST\s*\(`)
	defFuncRe    = regexp.MustCompile(`(?i)\bDEFAULT\s+(gen_random_uuid\(\)|now\(\))`)
	createTblRe  = regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+([a-zA-Z0-9_]+)\b`)
	createIdxRe  = regexp.MustCompile(`(?i)\bCREATE\s+(UNIQUE\s+)?INDEX\s+([a-zA-Z0-9_]+)\b`)
	isoTimeRe    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`)
)

func init() {
	RegisterSQLiteCompat()
}

// RegisterSQLiteCompat registers the "pgsqlite" driver that translates PostgreSQL
// schema-qualified SQL and dialect constructs to SQLite 3 for local/test execution.
func RegisterSQLiteCompat() {
	registerOnce.Do(func() {
		sqlite.MustRegisterScalarFunction("gen_random_uuid", 0, func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			b := make([]byte, 16)
			if _, err := rand.Read(b); err != nil {
				return nil, err
			}
			b[6] = (b[6] & 0x0f) | 0x40
			b[8] = (b[8] & 0x3f) | 0x80
			return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
		})
		sqlite.MustRegisterScalarFunction("now", 0, func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			return time.Now().UTC().Format(time.RFC3339Nano), nil
		})
		sqlite.MustRegisterScalarFunction("greatest", 2, func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			a, b := toInt64(args[0]), toInt64(args[1])
			if a >= b {
				return a, nil
			}
			return b, nil
		})
		sqlite.MustRegisterScalarFunction("least", 2, func(ctx *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			a, b := toInt64(args[0]), toInt64(args[1])
			if a <= b {
				return a, nil
			}
			return b, nil
		})
		sql.Register("pgsqlite", &pgSQLiteDriver{base: &sqlite.Driver{}})
	})
}

func toInt64(v driver.Value) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	default:
		return 0
	}
}

// RewriteSQL translates PostgreSQL DDL/DML into SQLite-compatible SQL while preserving string literals.
func RewriteSQL(query string) string {
	var out strings.Builder
	inQuote := false
	start := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '\'' {
			if !inQuote {
				out.WriteString(rewriteSegment(query[start:i]))
				start = i
				inQuote = true
			} else {
				if i+1 < len(query) && query[i+1] == '\'' {
					i++
					continue
				}
				out.WriteString(query[start : i+1])
				start = i + 1
				inQuote = false
			}
		}
	}
	if start < len(query) {
		if inQuote {
			out.WriteString(query[start:])
		} else {
			out.WriteString(rewriteSegment(query[start:]))
		}
	}
	return out.String()
}

func rewriteSegment(seg string) string {
	seg = schemaRe.ReplaceAllString(seg, "${1}__${2}")
	seg = paramRe.ReplaceAllString(seg, "?${1}")
	seg = ilikeRe.ReplaceAllString(seg, "LIKE")
	seg = forUpdateRe.ReplaceAllString(seg, "")
	seg = identityPKRe.ReplaceAllString(seg, "INTEGER PRIMARY KEY AUTOINCREMENT")
	seg = textArrRe.ReplaceAllString(seg, "TEXT NOT NULL DEFAULT '[]'")
	seg = castJsonbRe.ReplaceAllString(seg, "")
	seg = defFuncRe.ReplaceAllString(seg, "DEFAULT (${1})")
	seg = nowFuncRe.ReplaceAllString(seg, "strftime('%Y-%m-%dT%H:%M:%fZ','now')")
	seg = uuidFuncRe.ReplaceAllString(seg, "lower(hex(randomblob(4))||'-'||hex(randomblob(2))||'-4'||substr(hex(randomblob(2)),2)||'-'||substr('89ab',abs(random())%4+1,1)||substr(hex(randomblob(2)),2)||'-'||hex(randomblob(6)))")
	seg = greatestRe.ReplaceAllString(seg, "MAX(")
	seg = leastRe.ReplaceAllString(seg, "MIN(")
	seg = createTblRe.ReplaceAllString(seg, "CREATE TABLE IF NOT EXISTS ${1}")
	seg = createIdxRe.ReplaceAllString(seg, "CREATE ${1}INDEX IF NOT EXISTS ${2}")
	return seg
}

// ApplySchemaFile executes a PostgreSQL schema file (such as database/001_all_phases.sql) against db.
func ApplySchemaFile(ctx context.Context, db *sql.DB, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return ApplySchemaSQL(ctx, db, string(raw))
}

// ApplySchemaSQL executes raw DDL statements against db.
func ApplySchemaSQL(ctx context.Context, db *sql.DB, ddl string) error {
	lines := strings.Split(ddl, "\n")
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
			continue
		}
		if createSchRe.MatchString(trimmed) {
			continue
		}
		filtered = append(filtered, line)
	}
	full := strings.Join(filtered, "\n")
	stmts := strings.Split(full, ";")
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("executing DDL [%s]: %w", stmt, err)
		}
	}
	return nil
}

type pgSQLiteDriver struct {
	base driver.Driver
}

func (d *pgSQLiteDriver) Open(name string) (driver.Conn, error) {
	dsn := strings.TrimPrefix(name, "sqlite://")
	if !strings.Contains(dsn, "_pragma=") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn = dsn + sep + "_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	}
	c, err := d.base.Open(dsn)
	if err != nil {
		return nil, err
	}
	return &pgSQLiteConn{Conn: c}, nil
}

type pgSQLiteConn struct {
	driver.Conn
}

func (c *pgSQLiteConn) Prepare(query string) (driver.Stmt, error) {
	return c.Conn.Prepare(RewriteSQL(query))
}

func (c *pgSQLiteConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if pc, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return pc.PrepareContext(ctx, RewriteSQL(query))
	}
	return c.Prepare(query)
}

func (c *pgSQLiteConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if ec, ok := c.Conn.(driver.ExecerContext); ok {
		return ec.ExecContext(ctx, RewriteSQL(query), args)
	}
	stmt, err := c.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	if se, ok := stmt.(driver.StmtExecContext); ok {
		return se.ExecContext(ctx, args)
	}
	vals := make([]driver.Value, len(args))
	for i, a := range args {
		vals[i] = a.Value
	}
	return stmt.Exec(vals)
}

func (c *pgSQLiteConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if qc, ok := c.Conn.(driver.QueryerContext); ok {
		rows, err := qc.QueryContext(ctx, RewriteSQL(query), args)
		if err != nil {
			return nil, err
		}
		return &pgSQLiteRows{Rows: rows}, nil
	}
	stmt, err := c.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	if sq, ok := stmt.(driver.StmtQueryContext); ok {
		rows, err := sq.QueryContext(ctx, args)
		if err != nil {
			stmt.Close()
			return nil, err
		}
		return &pgSQLiteRows{Rows: rows}, nil
	}
	vals := make([]driver.Value, len(args))
	for i, a := range args {
		vals[i] = a.Value
	}
	rows, err := stmt.Query(vals)
	if err != nil {
		stmt.Close()
		return nil, err
	}
	return &pgSQLiteRows{Rows: rows}, nil
}

type pgSQLiteRows struct {
	driver.Rows
}

func (r *pgSQLiteRows) Next(dest []driver.Value) error {
	if err := r.Rows.Next(dest); err != nil {
		return err
	}
	for i, v := range dest {
		if s, ok := v.(string); ok && isoTimeRe.MatchString(s) {
			for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05"} {
				if t, err := time.Parse(layout, s); err == nil {
					dest[i] = t
					break
				}
			}
		}
	}
	return nil
}

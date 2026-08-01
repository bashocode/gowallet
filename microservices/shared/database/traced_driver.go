package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"sync"

	"github.com/go-sql-driver/mysql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	tracedDriverName = "mysql-otel"
	registerOnce     sync.Once
)

func initTracedDriver() {
	registerOnce.Do(func() {
		sql.Register(tracedDriverName, &tracedDriver{parent: &mysql.MySQLDriver{}})
	})
}

type tracedDriver struct {
	parent driver.Driver
}

func (d *tracedDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.parent.Open(name)
	if err != nil {
		return nil, err
	}
	return &tracedConn{conn: conn}, nil
}

type tracedConn struct {
	conn driver.Conn
}

func (c *tracedConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := c.conn.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &tracedStmt{stmt: stmt, query: query}, nil
}

func (c *tracedConn) Close() error {
	return c.conn.Close()
}

//nolint:staticcheck // SA1019: implementing driver.Conn interface fallback
func (c *tracedConn) Begin() (driver.Tx, error) {
	return c.conn.Begin()
}

func (c *tracedConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	tracer := otel.Tracer("db")
	ctx, span := tracer.Start(ctx, "DB BeginTx", trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()

	if connBegin, ok := c.conn.(driver.ConnBeginTx); ok {
		tx, err := connBegin.BeginTx(ctx, opts)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
			return nil, err
		}
		return &tracedTx{tx: tx}, nil
	}
	//nolint:staticcheck // SA1019: fallback for legacy drivers
	tx, err := c.conn.Begin()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	return &tracedTx{tx: tx}, nil
}

func (c *tracedConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if execer, ok := c.conn.(driver.ExecerContext); ok {
		tracer := otel.Tracer("db")
		ctx, span := tracer.Start(ctx, "DB Exec: "+truncateQuery(query),
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system", "mysql"),
				attribute.String("db.statement", query),
			),
		)
		defer span.End()

		res, err := execer.ExecContext(ctx, query, args)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return res, err
	}
	return nil, driver.ErrSkip
}

func (c *tracedConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if queryer, ok := c.conn.(driver.QueryerContext); ok {
		tracer := otel.Tracer("db")
		ctx, span := tracer.Start(ctx, "DB Query: "+truncateQuery(query),
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system", "mysql"),
				attribute.String("db.statement", query),
			),
		)
		defer span.End()

		rows, err := queryer.QueryContext(ctx, query, args)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return rows, err
	}
	return nil, driver.ErrSkip
}

func (c *tracedConn) Ping(ctx context.Context) error {
	if pinger, ok := c.conn.(driver.Pinger); ok {
		return pinger.Ping(ctx)
	}
	return nil
}

type tracedStmt struct {
	stmt  driver.Stmt
	query string
}

func (s *tracedStmt) Close() error {
	return s.stmt.Close()
}

func (s *tracedStmt) NumInput() int {
	return s.stmt.NumInput()
}

//nolint:staticcheck // SA1019: implementing driver.Stmt interface fallback
func (s *tracedStmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.stmt.Exec(args)
}

//nolint:staticcheck // SA1019: implementing driver.Stmt interface fallback
func (s *tracedStmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.stmt.Query(args)
}

func (s *tracedStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	if execer, ok := s.stmt.(driver.StmtExecContext); ok {
		tracer := otel.Tracer("db")
		ctx, span := tracer.Start(ctx, "DB StmtExec: "+truncateQuery(s.query),
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system", "mysql"),
				attribute.String("db.statement", s.query),
			),
		)
		defer span.End()

		res, err := execer.ExecContext(ctx, args)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return res, err
	}
	//nolint:staticcheck // SA1019: fallback for legacy drivers
	return s.stmt.Exec(namedToValues(args))
}

func (s *tracedStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if queryer, ok := s.stmt.(driver.StmtQueryContext); ok {
		tracer := otel.Tracer("db")
		ctx, span := tracer.Start(ctx, "DB StmtQuery: "+truncateQuery(s.query),
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system", "mysql"),
				attribute.String("db.statement", s.query),
			),
		)
		defer span.End()

		rows, err := queryer.QueryContext(ctx, args)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return rows, err
	}
	//nolint:staticcheck // SA1019: fallback for legacy drivers
	return s.stmt.Query(namedToValues(args))
}

type tracedTx struct {
	tx driver.Tx
}

func (t *tracedTx) Commit() error {
	return t.tx.Commit()
}

func (t *tracedTx) Rollback() error {
	return t.tx.Rollback()
}

func truncateQuery(q string) string {
	if len(q) > 60 {
		return q[:57] + "..."
	}
	return q
}

func namedToValues(args []driver.NamedValue) []driver.Value {
	values := make([]driver.Value, len(args))
	for i, arg := range args {
		values[i] = arg.Value
	}
	return values
}

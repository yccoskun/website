package services

import "database/sql"

// dbQuerier is satisfied by *sql.DB and *sql.Tx so import Apply can share one transaction.
type dbQuerier interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Prepare(query string) (*sql.Stmt, error)
}

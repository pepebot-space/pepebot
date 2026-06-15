//go:build mips || mipsle || mips64 || mips64le
// +build mips mipsle mips64 mips64le

package task

import "fmt"

const sqliteUnsupportedOnMIPSError = "SQLite task backend is not supported on MIPS architectures (modernc.org/sqlite unavailable)"

// SQLiteTaskStore stub for MIPS architectures where modernc SQLite is unavailable.
type SQLiteTaskStore struct{}

func NewSQLiteTaskStore(_ string) (*SQLiteTaskStore, error) {
	return nil, fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) Create(_ *Task) error {
	return fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) Get(_ string) (*Task, error) {
	return nil, fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) Update(_ *Task) error {
	return fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) Delete(_ string) error {
	return fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) List(_ TaskFilter) ([]*Task, error) {
	return nil, fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) Checkout(_, _ string, _ []string) (*Task, error) {
	return nil, fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) Move(_ string, _ TaskStatus) error {
	return fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) Approve(_, _, _ string) error {
	return fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) Reject(_, _, _ string) error {
	return fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) Stats() (map[TaskStatus]int, error) {
	return nil, fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) Cleanup(_, _ int) (int, error) {
	return 0, fmt.Errorf(sqliteUnsupportedOnMIPSError)
}

func (s *SQLiteTaskStore) Close() error {
	return nil
}

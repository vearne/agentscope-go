package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/vearne/agentscope-go/pkg/message"
)

// SQLMemory is the SQL-based implementation of memory storage.
// Currently only SQLite is supported. The SQL syntax uses SQLite-specific
// features such as "INSERT OR IGNORE" and "INSERT OR REPLACE".
type SQLMemory struct {
	db        *sql.DB
	sessionID string
	userID    string
	mu        sync.RWMutex
	initialized bool
}

// SQLMemoryOption is a function type for configuring SQLMemory.
type SQLMemoryOption func(*SQLMemory)

// WithSessionID sets the session ID for the memory.
func WithSQLSessionID(sessionID string) SQLMemoryOption {
	return func(sm *SQLMemory) {
		sm.sessionID = sessionID
	}
}

// WithSQLUserID sets the user ID for the memory.
func WithSQLUserID(userID string) SQLMemoryOption {
	return func(sm *SQLMemory) {
		sm.userID = userID
	}
}

// NewSQLMemory creates a new SQL-based memory storage.
func NewSQLMemory(db *sql.DB, opts ...SQLMemoryOption) *SQLMemory {
	sm := &SQLMemory{
		db:        db,
		sessionID: defaultSessionID,
		userID:    defaultUserID,
	}

	for _, opt := range opts {
		opt(sm)
	}

	return sm
}

// GetDB returns the underlying database connection.
func (sm *SQLMemory) GetDB() *sql.DB {
	return sm.db
}

// initTables creates the necessary tables if they don't exist.
func (sm *SQLMemory) initTables(ctx context.Context) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.initialized {
		return nil
	}

	tx, err := sm.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			msg_json TEXT NOT NULL,
			msg_index INTEGER NOT NULL,
			FOREIGN KEY (session_id) REFERENCES sessions(id)
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS message_marks (
			msg_id TEXT NOT NULL,
			mark TEXT NOT NULL,
			PRIMARY KEY (msg_id, mark),
			FOREIGN KEY (msg_id) REFERENCES messages(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS compressed_summaries (
			session_id TEXT PRIMARY KEY,
			summary TEXT NOT NULL,
			FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_messages_index ON messages(session_id, msg_index)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_message_marks_msg_id ON message_marks(msg_id)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_message_marks_mark ON message_marks(mark)
	`)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO users (id) VALUES (?)
	`, sm.userID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO sessions (id, user_id) VALUES (?, ?)
	`, sm.sessionID, sm.userID)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	sm.initialized = true
	return nil
}

// Add adds message(s) into the memory storage.
func (sm *SQLMemory) Add(ctx context.Context, msgs ...*message.Msg) error {
	return sm.AddWithMarks(ctx, msgs, nil)
}

// AddWithMarks adds message(s) into the memory storage with specified marks.
func (sm *SQLMemory) AddWithMarks(ctx context.Context, msgs []*message.Msg, marks []string) error {
	if len(msgs) == 0 {
		return nil
	}

	if err := sm.initTables(ctx); err != nil {
		return err
	}

	tx, err := sm.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	maxIndex, err := sm.getNextIndexTx(ctx, tx)
	if err != nil {
		return err
	}

	for i, msg := range msgs {
		msgData, err := json.Marshal(msg)
		if err != nil {
			return err
		}

		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO messages (id, session_id, msg_json, msg_index)
			VALUES (?, ?, ?, ?)
		`, msg.ID, sm.sessionID, string(msgData), maxIndex+int64(i))
		if err != nil {
			return err
		}

		for _, mark := range marks {
			_, err = tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO message_marks (msg_id, mark)
				VALUES (?, ?)
			`, msg.ID, mark)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// GetMessages returns all messages without filtering.
func (sm *SQLMemory) GetMessages() []*message.Msg {
	ctx := context.Background()
	msgs, _ := sm.GetMemory(ctx, "", "", false)
	return msgs
}

// GetMemory retrieves messages with optional mark filtering.
func (sm *SQLMemory) GetMemory(ctx context.Context, mark string, excludeMark string, prependSummary bool) ([]*message.Msg, error) {
	if err := sm.initTables(ctx); err != nil {
		return nil, err
	}

	query := `
		SELECT msg_json FROM messages
		WHERE session_id = ?
	`
	args := []interface{}{sm.sessionID}

	if mark != "" {
		query += `
			AND id IN (
				SELECT msg_id FROM message_marks WHERE mark = ?
			)
		`
		args = append(args, mark)
	}

	if excludeMark != "" {
		query += `
			AND id NOT IN (
				SELECT msg_id FROM message_marks WHERE mark = ?
			)
		`
		args = append(args, excludeMark)
	}

	query += " ORDER BY msg_index"

	rows, err := sm.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*message.Msg
	for rows.Next() {
		var msgData string
		if err := rows.Scan(&msgData); err != nil {
			return nil, err
		}

		var msg message.Msg
		if err := json.Unmarshal([]byte(msgData), &msg); err != nil {
			return nil, err
		}

		messages = append(messages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if prependSummary {
		var summary string
		err := sm.db.QueryRowContext(ctx, `
			SELECT summary FROM compressed_summaries WHERE session_id = ?
		`, sm.sessionID).Scan(&summary)

		if err == nil && summary != "" {
			result := make([]*message.Msg, 0, len(messages)+1)
			result = append(result, message.NewMsg("user", summary, "user"))
			result = append(result, messages...)
			return result, nil
		}
	}

	return messages, nil
}

// Clear clears all messages from the storage.
func (sm *SQLMemory) Clear(ctx context.Context) error {
	if err := sm.initTables(ctx); err != nil {
		return err
	}

	tx, err := sm.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		DELETE FROM message_marks
		WHERE msg_id IN (SELECT id FROM messages WHERE session_id = ?)
	`, sm.sessionID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM messages WHERE session_id = ?
	`, sm.sessionID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `
		DELETE FROM compressed_summaries WHERE session_id = ?
	`, sm.sessionID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Delete removes message(s) from the storage by their IDs.
func (sm *SQLMemory) Delete(ctx context.Context, msgIDs []string) (int, error) {
	if len(msgIDs) == 0 {
		return 0, nil
	}

	if err := sm.initTables(ctx); err != nil {
		return 0, err
	}

	tx, err := sm.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	placeholders := make([]string, len(msgIDs))
	args := make([]interface{}, len(msgIDs))
	for i, id := range msgIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("DELETE FROM message_marks WHERE msg_id IN (%s)", 
		strings.Join(placeholders, ", "))
	_, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	query = fmt.Sprintf("DELETE FROM messages WHERE id IN (%s)", 
		strings.Join(placeholders, ", "))
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// DeleteByMark removes messages from the memory by their marks.
func (sm *SQLMemory) DeleteByMark(ctx context.Context, marks []string) (int, error) {
	if len(marks) == 0 {
		return 0, nil
	}

	if err := sm.initTables(ctx); err != nil {
		return 0, err
	}

	tx, err := sm.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	placeholders := make([]string, len(marks))
	args := make([]interface{}, len(marks))
	for i, mark := range marks {
		placeholders[i] = "?"
		args[i] = mark
	}

	query := fmt.Sprintf(`
		SELECT msg_id FROM message_marks
		WHERE mark IN (%s)
	`, strings.Join(placeholders, ", "))

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var msgIDs []string
	for rows.Next() {
		var msgID string
		if err := rows.Scan(&msgID); err != nil {
			return 0, err
		}
		msgIDs = append(msgIDs, msgID)
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(msgIDs) == 0 {
		return 0, nil
	}

	deletedCount, err := sm.deleteMessagesTx(ctx, tx, msgIDs)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return deletedCount, nil
}

func (sm *SQLMemory) deleteMessagesTx(ctx context.Context, tx *sql.Tx, msgIDs []string) (int, error) {
	placeholders := make([]string, len(msgIDs))
	args := make([]interface{}, len(msgIDs))
	for i, id := range msgIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf("DELETE FROM message_marks WHERE msg_id IN (%s)", 
		strings.Join(placeholders, ", "))
	_, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	query = fmt.Sprintf("DELETE FROM messages WHERE id IN (%s)", 
		strings.Join(placeholders, ", "))
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// Size returns the number of messages in the storage.
func (sm *SQLMemory) Size() int {
	ctx := context.Background()
	if err := sm.initTables(ctx); err != nil {
		return 0
	}

	var count int
	err := sm.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM messages WHERE session_id = ?
	`, sm.sessionID).Scan(&count)

	if err != nil {
		return 0
	}

	return count
}

// ToStrList converts messages to a list of strings.
func (sm *SQLMemory) ToStrList() []string {
	ctx := context.Background()
	msgs, _ := sm.GetMemory(ctx, "", "", false)
	result := make([]string, 0, len(msgs))
	for _, msg := range msgs {
		result = append(result, msg.GetTextContent())
	}
	return result
}

// UpdateCompressedSummary updates the compressed summary.
func (sm *SQLMemory) UpdateCompressedSummary(ctx context.Context, summary string) error {
	if err := sm.initTables(ctx); err != nil {
		return err
	}

	_, err := sm.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO compressed_summaries (session_id, summary)
		VALUES (?, ?)
	`, sm.sessionID, summary)

	return err
}

// UpdateMessagesMark updates marks of messages.
func (sm *SQLMemory) UpdateMessagesMark(ctx context.Context, newMark string, oldMark string, msgIDs []string) (int, error) {
	if err := sm.initTables(ctx); err != nil {
		return 0, err
	}

	tx, err := sm.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	query := `
		SELECT id FROM messages
		WHERE session_id = ?
	`
	args := []interface{}{sm.sessionID}

	if len(msgIDs) > 0 {
		placeholders := make([]string, len(msgIDs))
		msgIDArgs := make([]interface{}, len(msgIDs))
		for i, id := range msgIDs {
			placeholders[i] = "?"
			msgIDArgs[i] = id
		}
		query += fmt.Sprintf(" AND id IN (%s)", 
			strings.Join(placeholders, ", "))
		args = append(args, msgIDArgs...)
	}

	if oldMark != "" {
		query += `
			AND id IN (SELECT msg_id FROM message_marks WHERE mark = ?)
		`
		args = append(args, oldMark)
	}

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var targetMsgIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		targetMsgIDs = append(targetMsgIDs, id)
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(targetMsgIDs) == 0 {
		return 0, nil
	}

	updatedCount := 0

	if newMark == "" {
		for _, msgID := range targetMsgIDs {
			result, err := tx.ExecContext(ctx, `
				DELETE FROM message_marks WHERE msg_id = ? AND mark = ?
			`, msgID, oldMark)
			if err != nil {
				return 0, err
			}
			affected, _ := result.RowsAffected()
			updatedCount += int(affected)
		}
	} else {
		if oldMark != "" {
			for _, msgID := range targetMsgIDs {
				_, err := tx.ExecContext(ctx, `
					DELETE FROM message_marks WHERE msg_id = ? AND mark = ?
				`, msgID, oldMark)
				if err != nil {
					return 0, err
				}
			}
		}

		for _, msgID := range targetMsgIDs {
			result, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO message_marks (msg_id, mark)
				VALUES (?, ?)
			`, msgID, newMark)
			if err != nil {
				return 0, err
			}
			affected, _ := result.RowsAffected()
			updatedCount += int(affected)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return updatedCount, nil
}

// getNextIndexTx gets the next message index within a transaction.
func (sm *SQLMemory) getNextIndexTx(ctx context.Context, tx *sql.Tx) (int64, error) {
	var maxIndex sql.NullInt64
	err := tx.QueryRowContext(ctx, `
		SELECT MAX(msg_index) FROM messages WHERE session_id = ?
	`, sm.sessionID).Scan(&maxIndex)

	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}

	if maxIndex.Valid {
		return maxIndex.Int64 + 1, nil
	}

	return 0, nil
}

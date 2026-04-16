package internal

import (
	"context"
	"database/sql"
	"real-time-chat/config"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

type Database struct {
	conn *sql.DB
}

type MessageRow struct {
	ID        uuid.UUID
	Sender    string
	Text      string
	CreatedAt time.Time
}

func NewDatabase(cfg *config.Config) (*Database, error) {
	// собрать connString из cfg
	// connString := `postgres://user:password@127.0.0.1:5433/urlshortener?sslmode=disable`
	connString := "postgres://" + cfg.DBUser + ":" + cfg.DBPassword + "@" + cfg.DBHost + ":" + cfg.DBPort + "/" + cfg.DBName + "?sslmode=disable"

	db, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(100)
	db.SetConnMaxLifetime(time.Minute)

	return &Database{conn: db}, nil
}

func (db *Database) CreateUser(ctx context.Context, username, password string) (uuid.UUID, error) {

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return uuid.Nil, err
	}

	id := uuid.New()

	_, err = db.conn.ExecContext(ctx,
		"INSERT INTO users (id, username, password_hash) VALUES ($1, $2, $3)",
		id, username, string(hash),
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (db *Database) GetUserByName(ctx context.Context, username string) (uuid.UUID, string, error) {

	query := `SELECT id, password_hash FROM users WHERE username = $1`

	var id uuid.UUID
	var hash string

	err := db.conn.QueryRowContext(ctx, query, username).Scan(
		&id,
		&hash,
	)

	if err == sql.ErrNoRows {
		return uuid.Nil, hash, nil
	}

	if err != nil {
		return uuid.Nil, hash, err
	}

	return id, hash, nil
}

func (db *Database) GetOrCreateRoom(ctx context.Context, name string) (uuid.UUID, error) {

	query := `SELECT id FROM rooms WHERE name = $1`

	var id uuid.UUID

	err := db.conn.QueryRowContext(ctx, query, name).Scan(
		&id,
	)

	if err == sql.ErrNoRows {
		// нужно создать roomu
		id = uuid.New()
		_, err = db.conn.ExecContext(ctx,
			"INSERT INTO rooms (id, name) VALUES ($1, $2)",
			id, name,
		)
	}

	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (db *Database) SaveMessage(ctx context.Context, roomID uuid.UUID, senderID uuid.UUID, text string) error {

	id := uuid.New()
	_, err := db.conn.ExecContext(ctx,
		"INSERT INTO msg (id, room_id, sender_id, text) VALUES ($1, $2, $3, $4)",
		id, roomID, senderID, text,
	)
	if err != nil {
		return err
	}

	return nil

}

func (db *Database) GetHistory(ctx context.Context, roomID uuid.UUID, limit int) ([]MessageRow, error) {

	query := `SELECT m.id, u.username, m.text, m.created_at 
	FROM msg m
	JOIN users u ON m.sender_id = u.id
	WHERE m.room_id = $1
	ORDER BY m.created_at DESC
	LIMIT $2`

	rows, err := db.conn.QueryContext(ctx, query, roomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []MessageRow
	for rows.Next() {
		var m MessageRow
		if err := rows.Scan(&m.ID, &m.Sender, &m.Text, &m.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	return messages, rows.Err()

}

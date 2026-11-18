package main

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type App struct {
	ID          int64
	Name        string
	URL         string
	Icon        string // "" (letter fallback), external http(s) URL, or "/uploads/<file>"
	Description string
	Position    int
	NewTab      bool
}

type Store struct{ db *sql.DB }

const schema = `
CREATE TABLE IF NOT EXISTS apps (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	name        TEXT    NOT NULL,
	url         TEXT    NOT NULL,
	icon        TEXT    NOT NULL DEFAULT '',
	description TEXT    NOT NULL DEFAULT '',
	position    INTEGER NOT NULL DEFAULT 0,
	new_tab     INTEGER NOT NULL DEFAULT 1,
	created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

func OpenStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

const cols = `id, name, url, icon, description, position, new_tab`

func scanApp(row interface{ Scan(...any) error }) (App, error) {
	var a App
	err := row.Scan(&a.ID, &a.Name, &a.URL, &a.Icon, &a.Description, &a.Position, &a.NewTab)
	return a, err
}

func (s *Store) List() ([]App, error) {
	rows, err := s.db.Query(`SELECT ` + cols + ` FROM apps ORDER BY position, name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var apps []App
	for rows.Next() {
		a, err := scanApp(rows)
		if err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

func (s *Store) Get(id int64) (App, error) {
	return scanApp(s.db.QueryRow(`SELECT `+cols+` FROM apps WHERE id = ?`, id))
}

func (s *Store) NextPosition() int {
	var n int
	s.db.QueryRow(`SELECT COALESCE(MAX(position), 0) + 10 FROM apps`).Scan(&n)
	return n
}

func (s *Store) Create(a *App) error {
	res, err := s.db.Exec(`INSERT INTO apps (name, url, icon, description, position, new_tab) VALUES (?, ?, ?, ?, ?, ?)`,
		a.Name, a.URL, a.Icon, a.Description, a.Position, a.NewTab)
	if err != nil {
		return err
	}
	a.ID, err = res.LastInsertId()
	return err
}

func (s *Store) Update(a *App) error {
	_, err := s.db.Exec(`UPDATE apps SET name = ?, url = ?, icon = ?, description = ?, position = ?, new_tab = ? WHERE id = ?`,
		a.Name, a.URL, a.Icon, a.Description, a.Position, a.NewTab, a.ID)
	return err
}

func (s *Store) Delete(id int64) error {
	_, err := s.db.Exec(`DELETE FROM apps WHERE id = ?`, id)
	return err
}

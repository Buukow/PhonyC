package store

import (
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

func (s *Store) AdminCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM admin_user`).Scan(&n)
	return n, err
}

func (s *Store) GetAdminByUsername(username string) (*AdminUser, error) {
	row := s.db.QueryRow(`SELECT id, username, password_hash, created_at, updated_at FROM admin_user WHERE username = ?`, username)
	return scanAdmin(row)
}

func (s *Store) GetAdminByID(id int64) (*AdminUser, error) {
	row := s.db.QueryRow(`SELECT id, username, password_hash, created_at, updated_at FROM admin_user WHERE id = ?`, id)
	return scanAdmin(row)
}

func scanAdmin(row *sql.Row) (*AdminUser, error) {
	var a AdminUser
	var cAt, uAt string
	if err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &cAt, &uAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	a.CreatedAt = parseTime(cAt)
	a.UpdatedAt = parseTime(uAt)
	return &a, nil
}

func (s *Store) CreateAdmin(username, passwordHash string) (*AdminUser, error) {
	n, err := s.AdminCount()
	if err != nil {
		return nil, err
	}
	if n > 0 {
		return nil, fmt.Errorf("admin already initialized")
	}
	now := formatTime(nowUTC())
	res, err := s.db.Exec(`INSERT INTO admin_user(username, password_hash, created_at, updated_at) VALUES(?,?,?,?)`,
		username, passwordHash, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetAdminByID(id)
}

func (s *Store) UpdateAdminPassword(id int64, passwordHash string) error {
	now := formatTime(nowUTC())
	res, err := s.db.Exec(`UPDATE admin_user SET password_hash=?, updated_at=? WHERE id=?`, passwordHash, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateAdminUsername(id int64, username string) error {
	now := formatTime(nowUTC())
	res, err := s.db.Exec(`UPDATE admin_user SET username=?, updated_at=? WHERE id=?`, username, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

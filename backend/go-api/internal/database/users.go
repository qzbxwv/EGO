package database

import (
	"egobackend/internal/models"
)

func (db *DB) CreateUser(username, hashedPassword string) (*models.User, error) {
	query := `INSERT INTO users (username, hashed_password) VALUES ($1, $2) RETURNING *`

	var newUser models.User
	err := db.Get(&newUser, query, username, hashedPassword)
	if err != nil {
		return nil, err
	}

	return &newUser, nil
}

func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE username = $1`

	err := db.Get(&user, query, username)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (db *DB) UpdateUserRole(userID int, newRole string) error {
	query := `UPDATE users SET role = $1 WHERE id = $2`
	_, err := db.Exec(query, newRole, userID)
	return err
}

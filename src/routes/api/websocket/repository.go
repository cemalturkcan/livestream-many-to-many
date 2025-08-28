package websocket

import (
	"context"
	"livestream/app/database"
)

const getUserByIdQuery = `
SELECT id, username, avatar FROM usr WHERE id = $1
`

func GetUserById(id string) (*User, error) {
	var user User
	err := database.DB.QueryRow(context.Background(), getUserByIdQuery, id).Scan(&user.Id, &user.Username, &user.Avatar)
	return &user, err
}

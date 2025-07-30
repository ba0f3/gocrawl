package user

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"github.com/google/uuid"

	"gocrawl/internal/db"
)

func Register(database *db.Database, username, password string) (*db.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	apiKey := uuid.New().String()

	user := &db.User{
		Username: username,
		Password: string(hashedPassword),
		APIKey:   apiKey,
	}

	err = database.CreateUser(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func Login(database *db.Database, username, password string) (*db.User, error) {
	user, err := database.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return user, nil
}

func GetUserByAPIKey(database *db.Database, apiKey string) (*db.User, error) {
	return database.GetUserByAPIKey(apiKey)
}

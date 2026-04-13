package user

import (
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"gocrawl/internal/db"
)

func Register(database db.Store, username, password string) (*db.User, error) {
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

// dummyHash is a precomputed bcrypt hash (cost=10) of "dummy" used to prevent timing attacks.
var dummyHash = []byte("$2a$10$nYnTv04n8VisWw7tu6Z0r.5ddMBTljJOLbyXXGC9qFH9OdM1SF/6y")

func Login(database db.Store, username, password string) (*db.User, error) {
	user, err := database.GetUserByUsername(username)
	if err != nil {
		// Prevent timing attack: perform a dummy comparison when user is not found.
		// 🛡️ Sentinel: Prevents username enumeration by making invalid usernames take the same time to process.
		bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return nil, fmt.Errorf("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return user, nil
}

func GetUserByAPIKey(database db.Store, apiKey string) (*db.User, error) {
	return database.GetUserByAPIKey(apiKey)
}

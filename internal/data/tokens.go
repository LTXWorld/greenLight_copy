package data

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"github.com/LTXWorld/greenLight_copy/internal/validator"
	"time"
)

// Define constants for the token scope,such as activation,authentication
const (
	ScopeActivation     = "activation"
	ScopeAuthentication = "authentication"
)

// 要当做JSON响应传回
type Token struct {
	Plaintext string    `json:"token"`
	Hash      []byte    `json:"-"`
	UserID    int64     `json:"-"`
	Expiry    time.Time `json:"expiry"`
	Scope     string    `json:"-"`
}

func generateToken(userID int64, ttl time.Duration, scope string) (*Token, error) {
	// We add the provided ttl duration parameter to the current time to get expiry time
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	// Initialize a zero-valued byte slice with a length of 16 bytes
	randomBytes := make([]byte, 16)

	// Use the Read() function from crypto/rand to fill the byte slice with random bytes CSPRNG
	_, err := rand.Read(randomBytes)
	if err != nil {
		return nil, err
	}

	// 将字节切片编码为32为底的字符串并赋值给token的明文字段，同时也是在用户邮件中展示的
	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(randomBytes)

	// Generate a SHA-256 hash of the plaintext token string
	// 这个是我们存放在数据库中的，同时要将得到的array转换为hash slice
	hash := sha256.Sum256([]byte(token.Plaintext))
	token.Hash = hash[:]

	return token, nil
}

func ValidateTokenPlaintext(v *validator.Validator, tokenPlaintext string) {
	v.Check(tokenPlaintext != "", "token", "must be provided")
	v.Check(len(tokenPlaintext) == 26, "token", "must be 26 bytes long")
}

// Define the TokenModel type
type TokenModel struct {
	DB *sql.DB
}

// New creates a new Token and inserts the data in the tokens table
func (m TokenModel) New(userID int64, ttl time.Duration, scope string) (*Token, error) {
	token, err := generateToken(userID, ttl, scope)
	if err != nil {
		return nil, err
	}

	err = m.Insert(token)
	return token, err
}

// Insert adds the data for a specific token to the tokens table
func (m TokenModel) Insert(token *Token) error {
	query := `
			INSERT INTO tokens (hash, user_id, expiry, scope)
			VALUES ($1, $2, $3, $4)`
	args := []interface{}{token.Hash, token.UserID, token.Expiry, token.Scope}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, args...)
	return err
}

// 删除指定id和scope的tokens
func (m TokenModel) DeleteAllForUser(scope string, userID int64) error {
	query := `DELETE FROM tokens WHERE scope = $1 AND user_id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := m.DB.ExecContext(ctx, query, scope, userID)
	return err
}

package data

// 我们检查邮件是否重复存在问题，enumeration attacks
// 黑客可以通过请求来验证某个邮件是否存在，以此获取用户的邮件这一敏感信息
// 但是一些知名服务也没有防止用户枚举，给用户带来额外的麻烦要比隐私风险更糟糕
import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"github.com/LTXWorld/greenLight_copy/internal/validator"
	"golang.org/x/crypto/bcrypt"
	"time"
)

// Define a custom ErrDuplicateEmail error
var (
	ErrDuplicateEmail = errors.New("duplicate email")
	AnonymousUser     = &User{}
)

// We ignore the password and version during the JSON
type User struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  password  `json:"-"`
	Activated bool      `json:"activated"`
	Version   int       `json:"-"`
}

// Check if a User instance is the AnonymousUser
func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

// 明文密码和hash后的密码
type password struct {
	plaintext *string
	hash      []byte
}

// Set 将明文密码转换为哈希加密后的密码
func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword
	p.hash = hash

	return nil
}

// Matches 将提供的明文密码与存储的hash密码进行比较
func (p *password) Matches(plaintextPassword string) (bool, error) {
	// 使用与我们要比较的哈希字符串中相同的盐值和成本参数对提供的密码进行重新哈希
	// 然后再调用sutil.ConstantTimeCompare()将两个哈希值进行比较
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}

type UserModel struct {
	DB *sql.DB
}

// Insert 插入时注意检查email重复
func (m UserModel) Insert(user *User) error {
	query := `
		INSERT INTO users (name, email, password_hash, activated)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, version`
	args := []interface{}{user.Name, user.Email, user.Password.hash, user.Activated}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// err:如果email出现重复
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt, &user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		default:
			return err
		}
	}

	return nil
}

func (m UserModel) GetByEmail(email string) (*User, error) {
	query := `
			SELECT id, created_at, name, email, password_hash, activated, version
			FROM users
			WHERE email = $1`
	var user User
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := m.DB.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &user, nil
}

// Update 根据特定id和version（防止数据竞争）来进行更新
func (m UserModel) Update(user *User) error {
	query := `
			UPDATE users
			SET name = $1, email = $2, password_hash = $3, activated = $4, version = version + 1
			WHERE id = $5 AND version = $6
			RETURNING version`
	args := []interface{}{
		user.Name,
		user.Email,
		user.Password.hash,
		user.Activated,
		user.ID,
		user.Version,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.Version)
	if err != nil {
		switch {
		case err.Error() == `pq: duplicate key value violates unique constraint "users_email_key"`:
			return ErrDuplicateEmail
		case errors.Is(err, sql.ErrNoRows):
			return ErrEditConflict
		default:
			return err
		}
	}

	return nil
}

// ValidateEmail 验证邮件格式
func ValidateEmail(v *validator.Validator, email string) {
	v.Check(email != "", "email", "must be provided")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "must be a valid email address")
}

// ValidatePasswordPlaintext 验证用户传来的明文密码的格式
func ValidatePasswordPlaintext(v *validator.Validator, password string) {
	v.Check(password != "", "password", "must be provided")
	v.Check(len(password) >= 8, "password", "must be at least 8 bytes long")
	v.Check(len(password) <= 72, "password", "must not be more than 72 bytes long")
}

// ValidateUser 检查用户名，密码，邮件是否满足格式要求
func ValidateUser(v *validator.Validator, user *User) {
	v.Check(user.Name != "", "name", "must be provided")
	v.Check(len(user.Name) <= 500, "name", "must not be more than 500 bytes long")

	// Call the standalone ValidateEmail() helper
	ValidateEmail(v, user.Email)

	if user.Password.plaintext != nil {
		ValidatePasswordPlaintext(v, *user.Password.plaintext)
	}

	// 这里跟用户的输入没关系，来自于可能发生在代码逻辑中的错误
	if user.Password.hash == nil {
		panic("missing password hash for user")
	}
}

// GetForToken 通过令牌类型和明文令牌来获取用户信息
func (m UserModel) GetForToken(tokenScope, tokenPlaintext string) (*User, error) {
	// 先将用户传来的明文token进行加密
	tokenHash := sha256.Sum256([]byte(tokenPlaintext))

	// SQL query，根据id进行内连接
	query := `SELECT users.id, users.created_at, users.name, users.email, users.password_hash,
				users.activated, users.version
				FROM users
				INNER JOIN tokens
				ON users.id = tokens.user_id
				WHERE tokens.hash = $1
				AND tokens.scope = $2
				AND tokens.expiry > $3`

	args := []interface{}{tokenHash[:], tokenScope, time.Now()}

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute the query
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
		&user.Version,
	)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}

	return &user, nil
}

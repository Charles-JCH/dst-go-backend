package crypto

import (
	"golang.org/x/crypto/bcrypt"
)

type PasswordHasher interface {
	HashPassword(password string) (string, error)
	ComparePassword(hashedPassword string, password string) bool
}

type bcryptHasher struct {
	cost int
}

func NewPasswordHasher() PasswordHasher {
	return &bcryptHasher{cost: bcrypt.DefaultCost}
}

// HashPassword 生成密码哈希
func (b *bcryptHasher) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), b.cost)
	return string(bytes), err
}

// ComparePassword 校验明文密码与密码哈希是否一致
func (b *bcryptHasher) ComparePassword(hashedPassword string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

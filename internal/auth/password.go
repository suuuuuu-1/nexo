package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword 只保存不可逆的密码哈希，数据库中不保存明文密码。
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// ComparePassword 使用 bcrypt 校验用户输入的密码是否匹配已有哈希。
func ComparePassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

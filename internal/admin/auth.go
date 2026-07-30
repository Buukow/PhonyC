package admin

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/phonyg/phonyg/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	Store    *store.Store
	Secret   []byte
	TTLHours int
}

type claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func EnsureJWTSecret(path, envSecret string) ([]byte, error) {
	if s := strings.TrimSpace(envSecret); s != "" {
		return []byte(s), nil
	}
	if b, err := os.ReadFile(path); err == nil {
		b = []byte(strings.TrimSpace(string(b)))
		if len(b) > 0 {
			return b, nil
		}
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	sec := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(sec), 0o600); err != nil {
		return nil, err
	}
	return []byte(sec), nil
}

func HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func (a *Auth) IssueToken(admin *store.AdminUser) (string, error) {
	ttl := a.TTLHours
	if ttl <= 0 {
		ttl = 24
	}
	c := claims{
		Username: admin.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(admin.ID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(ttl) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(a.Secret)
}

func (a *Auth) Parse(token string) (*claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &claims{}, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return a.Secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := parsed.Claims.(*claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}

func (a *Auth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		tok := strings.TrimSpace(h[7:])
		cl, err := a.Parse(tok)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		id, _ := strconv.ParseInt(cl.Subject, 10, 64)
		c.Set("admin_id", id)
		c.Set("admin_username", cl.Username)
		c.Next()
	}
}

func RandomAPIKey() string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	return "sk-" + hex.EncodeToString(b)
}

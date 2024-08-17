package utils

import (
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
)



var jwtKey = []byte(os.Getenv("JWT_SECRET"))

func GenerateJWT(username string, role string) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["username"] = username
	claims["role"] = role
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix()

	t, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	return t, nil
}


func ValidateToken(tokenStr string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})

	if err != nil || !token.Valid {
		return nil, err
	}

	return token, nil
}

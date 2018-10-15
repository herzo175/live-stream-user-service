package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

type EncodedClaims map[string]interface{}

type TokenResponse struct {
	Token string `json:"token"`
}

func (claims EncodedClaims) Valid() error {
	tokenTime, err := time.Parse(time.RFC3339, claims["exp"].(string))

	if err != nil {
		return err
	}

	if time.Now().After(tokenTime) {
		return errors.New("Token has expired")
	}

	return nil
}

func IsAuthenticated(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				http.Error(w, "Must provide a bearer token", 401)
				log.Println(r)
			}
		}()

		token := strings.Split(r.Header.Get("Authorization"), "Bearer ")[1]
		tokenBody := make(map[string]interface{})
		err := ValidateToken(token, &tokenBody)

		if err != nil {
			http.Error(w, "Must provide valid bearer token", 403)
			log.Println(err)
		} else {
			// TODO: find way to allow user to pass in object to serialize to
			for k, v := range tokenBody {
				r.Header.Add(fmt.Sprintf("Token-Data.%s", k), fmt.Sprintf("%v", v))
			}

			next.ServeHTTP(w, r)
		}
	}
}

// data should encode as json
func GenerateToken(data interface{}) (tokenResponse TokenResponse, err error) {
	bytes, err := json.Marshal(data)

	if err != nil {
		return tokenResponse, err
	}

	claims := make(EncodedClaims)
	err = json.Unmarshal(bytes, &claims)

	if err != nil {
		return tokenResponse, err
	}

	// TODO: make time configurable?
	claims["exp"] = time.Now().Add(time.Hour * 8)
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)

	// TODO: move signing string to config
	tokenResponse = TokenResponse{}
	tokenString, err := token.SignedString([]byte("shhhh"))

	if err != nil {
		return tokenResponse, err
	}

	tokenResponse.Token = tokenString
	return tokenResponse, nil
}

// data should be a pointer to a json struct
func ValidateToken(tokenString string, data interface{}) error {
	claims := make(EncodedClaims)
	token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (interface{}, error) {
		// TODO: move signing string to config
		return []byte("shhhh"), nil
	})

	if err != nil {
		return err
	}

	if !token.Valid {
		return errors.New("Token is invalid")
	}

	bytes, err := json.Marshal(claims)

	if err != nil {
		return err
	}

	err = json.Unmarshal(bytes, data)

	if err != nil {
		return err
	}

	return nil
}

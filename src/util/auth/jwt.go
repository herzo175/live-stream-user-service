package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

type EncodedClaims map[string]interface{}

type TokenResponse struct {
	Token string `json:"token"`
}

type AuthHandlerFunc func(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{})

func (f AuthHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
	f(w, r, tokenBodyPointer)
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

// tokenBodySchema should be a struct to deserialize to
func IsAuthenticated(tokenBodySchema interface{}, next AuthHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bearerStrings := strings.Split(r.Header.Get("Authorization"), "Bearer ")

		if len(bearerStrings) != 2 {
			http.Error(w, "Must provide a bearer token", 401)
			return
		}

		token := bearerStrings[1]
		err := ValidateToken(token, tokenBodySchema)

		if err != nil {
			http.Error(w, "Must provide valid bearer token", 403)
			log.Println(err)
		} else {
			next.ServeHTTP(w, r, tokenBodySchema)
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

	// TODO: make time configurable
	claims["exp"] = time.Now().Add(time.Hour * 8)
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)

	// NOTE: use signing string file?
	tokenResponse = TokenResponse{}
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SIGNING_STRING")))

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
		return []byte(os.Getenv("JWT_SIGNING_STRING")), nil
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

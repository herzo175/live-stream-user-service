package auth

import (
	"fmt"
	"net/http"
	"os"
)

type Permission struct {
	Service string
	Role    string
}

type TokenBody interface {
	HasPermission(role, service string) bool
}

func IsAuthorized(roles []string, tokenBodySchema TokenBody, next AuthHandlerFunc) http.HandlerFunc {
	permissions := []Permission{}

	for _, r := range roles {
		permissions = append(permissions, Permission{Service: os.Getenv("SERVICE_NAME"), Role: r})
	}

	return IsAuthorizedService(permissions, tokenBodySchema, next)
}

// NOTE: will move out if need for seperate identity service
// use in case a route should be used
func IsAuthorizedService(allowedPerms []Permission, tokenBodySchema TokenBody, next AuthHandlerFunc) http.HandlerFunc {
	// return func(w http.ResponseWriter, r *http.Request) {
	// 	bearerStrings := strings.Split(r.Header.Get("Authorization"), "Bearer ")

	// 	if len(bearerStrings) != 2 {
	// 		http.Error(w, "Must provide a bearer token", 401)
	// 		return
	// 	}

	// 	token := bearerStrings[1]
	// 	err := ValidateToken(token, tokenBodySchema)

	// 	if err != nil {
	// 		http.Error(w, "Must provide valid bearer token", 403)
	// 		log.Println(err)
	// 		return
	// 	}

	return IsAuthenticated(tokenBodySchema, func(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
		token := *tokenBodyPointer.(*TokenBody)

		for _, perm := range allowedPerms {
			if token.HasPermission(perm.Service, perm.Role) {
				next.ServeHTTP(w, r, tokenBodyPointer)
				return
			}
		}

		http.Error(w, fmt.Sprintf("Bearer token is unauthorized for %s %s", r.Method, r.URL), 403)
	})
}

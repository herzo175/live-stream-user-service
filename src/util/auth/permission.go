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

func IsAuthorized(roles []string, tokenBodySchema TokenBody, signingString string, next AuthHandlerFunc) http.HandlerFunc {
	permissions := []Permission{}

	for _, r := range roles {
		permissions = append(permissions, Permission{Service: os.Getenv("SERVICE_NAME"), Role: r})
	}

	return IsAuthorizedService(permissions, tokenBodySchema, signingString, next)
}

// NOTE: will move out if need for seperate identity service
// use in case a route should be used
func IsAuthorizedService(allowedPerms []Permission, tokenBodySchema TokenBody, signingString string, next AuthHandlerFunc) http.HandlerFunc {
	return IsAuthenticated(tokenBodySchema, signingString, func(w http.ResponseWriter, r *http.Request, tokenBodyPointer interface{}) {
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

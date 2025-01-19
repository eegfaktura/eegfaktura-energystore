package middleware

import (
	"net/http"
)

func ProtectApp(handler JWTHandlerFunc) http.HandlerFunc {
	return verifyRequest(handler)
}

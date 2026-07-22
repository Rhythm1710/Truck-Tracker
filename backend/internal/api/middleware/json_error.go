package middleware

import (
	"encoding/json"
	"net/http"
)

// WriteJSONError writes the canonical {"error": "..."} body. Kept in the
// middleware package so non-handler middleware (auth, ratelimit) can use it
// without importing the api package and creating a cycle.
func WriteJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

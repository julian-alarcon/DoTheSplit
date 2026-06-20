package middleware

import (
	"encoding/json"
	"net/http"
)

// writeJSONError emits the same {code,message} error envelope the handlers use.
// It lives here because middleware can't import the handlers package (that would
// be a cycle), yet needs to reject requests with an identical body shape.
func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: message})
}

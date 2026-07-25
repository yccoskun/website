// Package response provides the JSON envelope every API endpoint returns:
// {"data": <payload or null>, "error": <string or null>}.
package response

import (
	"encoding/json"
	"log"
	"net/http"
)

type envelope struct {
	Data  any     `json:"data"`
	Error *string `json:"error"`
}

// JSON writes a success envelope with the given payload.
func JSON(w http.ResponseWriter, status int, data any) {
	write(w, status, envelope{Data: data})
}

// Error writes a failure envelope with a null data field.
func Error(w http.ResponseWriter, status int, msg string) {
	write(w, status, envelope{Error: &msg})
}

func write(w http.ResponseWriter, status int, env envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(env); err != nil {
		log.Printf("response: encode failed: %v", err)
	}
}

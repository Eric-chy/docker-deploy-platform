package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
)

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func Error(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

func Decode(w http.ResponseWriter, r *http.Request, limit int64, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid request body")
	}
	return nil
}

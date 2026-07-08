package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lib/pq"
)

// Sentinelles — comparées ensuite avec errors.Is, jamais par égalité de string.
var (
	ErrNotFound     = errors.New("ressource introuvable")
	ErrValidation   = errors.New("requête invalide")
	ErrConflict     = errors.New("conflit sur la ressource")
	ErrUnauthorized = errors.New("authentification requise")
)

type apiErrorBody struct {
	Error string `json:"error"`
}

// writeError est LE point unique de mapping erreur → code HTTP.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrValidation):
		status = http.StatusBadRequest
	case errors.Is(err, ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, context.DeadlineExceeded):
		status = http.StatusGatewayTimeout
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(apiErrorBody{Error: err.Error()})
}

// isUniqueViolation utilise errors.As pour extraire l'erreur concrète du driver lib/pq.
func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505" // unique_violation
	}
	return false
}

// isForeignKeyViolation détecte une violation de contrainte de clé étrangère.
func isForeignKeyViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23503" // foreign_key_violation
	}
	return false
}

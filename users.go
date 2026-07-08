package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func validateNewUser(u User) error {
	if strings.TrimSpace(u.Pseudo) == "" {
		return fmt.Errorf("pseudo requis: %w", ErrValidation)
	}
	return nil
}

func createUser(ctx context.Context, db *sql.DB, u User) (*User, error) {
	if err := validateNewUser(u); err != nil {
		return nil, err
	}
	return insertUser(ctx, db, u)
}

func updateUser(ctx context.Context, db *sql.DB, u User) (*User, error) {
	if err := validateNewUser(u); err != nil {
		return nil, err
	}
	if err := updateUserRow(ctx, db, u); err != nil {
		return nil, err
	}
	return selectUser(ctx, db, u.ID)
}

func createUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var u User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			writeError(w, fmt.Errorf("corps JSON invalide: %w", ErrValidation))
			return
		}
		created, err := createUser(r.Context(), db, u)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	}
}

func getUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		u, err := selectUser(r.Context(), db, id)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(u)
	}
}

func updateUserHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		var u User
		if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
			writeError(w, fmt.Errorf("corps JSON invalide: %w", ErrValidation))
			return
		}
		u.ID = id
		updated, err := updateUser(r.Context(), db, u)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updated)
	}
}

// RegisterUserRoutes enregistre les routes utilisateurs, compétences et évaluations.
func RegisterUserRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/users/{id}", getUserHandler(db))
	mux.HandleFunc("PUT /api/users/{id}", updateUserHandler(db))
}

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

func validateSkills(skills []Skill) error {
	for _, s := range skills {
		if !NiveauxValides[s.Niveau] {
			return fmt.Errorf("niveau invalide %q: %w", s.Niveau, ErrValidation)
		}
	}
	return nil
}

func replaceUserSkills(ctx context.Context, db *sql.DB, userID int, skills []Skill) error {
	if err := validateSkills(skills); err != nil {
		return err
	}
	return replaceSkillsRows(ctx, db, userID, skills)
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

func getUserSkillsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		skills, err := selectSkills(r.Context(), db, id)
		if err != nil {
			writeError(w, err)
			return
		}
		if skills == nil {
			skills = []Skill{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(skills)
	}
}

func putUserSkillsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		var skills []Skill
		if err := json.NewDecoder(r.Body).Decode(&skills); err != nil {
			writeError(w, fmt.Errorf("corps JSON invalide: %w", ErrValidation))
			return
		}
		if err := replaceUserSkills(r.Context(), db, id, skills); err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(skills)
	}
}

// RegisterUserRoutes enregistre les routes utilisateurs, compétences et évaluations.
func RegisterUserRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/users/{id}", getUserHandler(db))
	mux.HandleFunc("PUT /api/users/{id}", updateUserHandler(db))
	mux.HandleFunc("GET /api/users/{id}/skills", getUserSkillsHandler(db))
	mux.HandleFunc("PUT /api/users/{id}/skills", putUserSkillsHandler(db))
	mux.HandleFunc("GET /api/users/{id}/reviews", getUserReviewsHandler(db))
	mux.HandleFunc("GET /api/services/{id}/reviews", getServiceReviewsHandler(db))
	mux.HandleFunc("POST /api/exchanges/{id}/review", createReviewHandler(db))
}

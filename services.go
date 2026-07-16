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

func validateService(s Service, hasSkill bool) error {
	if !CategoriesValides[s.Categorie] {
		return fmt.Errorf("catégorie invalide %q: %w", s.Categorie, ErrValidation)
	}
	if strings.TrimSpace(s.Titre) == "" {
		return fmt.Errorf("titre requis: %w", ErrValidation)
	}
	if !hasSkill {
		return fmt.Errorf("compétence non déclarée par l'utilisateur: %w", ErrValidation)
	}
	return nil
}

func createService(ctx context.Context, db *sql.DB, s Service) (*Service, error) {
	has, err := userHasSkill(ctx, db, s.ProviderID, s.Categorie)
	if err != nil {
		return nil, err
	}
	if err := validateService(s, has); err != nil {
		return nil, err
	}
	return insertServiceRow(ctx, db, s)
}

func updateService(ctx context.Context, db *sql.DB, s Service) (*Service, error) {
	// Ownership vérifié avant toute validation, pour ne jamais renseigner un tiers non autorisé.
	existing, err := fetchService(ctx, db, s.ID)
	if err != nil {
		return nil, err
	}
	if existing.ProviderID != s.ProviderID {
		return nil, fmt.Errorf("service %d: %w", s.ID, ErrNotFound)
	}
	has, err := userHasSkill(ctx, db, s.ProviderID, s.Categorie)
	if err != nil {
		return nil, err
	}
	if err := validateService(s, has); err != nil {
		return nil, err
	}
	n, err := updateServiceRow(ctx, db, s)
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, fmt.Errorf("service %d: %w", s.ID, ErrNotFound)
	}
	return fetchService(ctx, db, s.ID)
}

func deleteService(ctx context.Context, db *sql.DB, serviceID, providerID int) error {
	hasActive, err := hasActiveExchangeForService(ctx, db, serviceID)
	if err != nil {
		return err
	}
	if hasActive {
		return fmt.Errorf("échange en cours sur le service %d: %w", serviceID, ErrConflict)
	}
	n, err := deleteServiceRow(ctx, db, serviceID, providerID)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("service %d: %w", serviceID, ErrNotFound)
	}
	return nil
}

func createServiceHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerID, ok := GetUserIDFromContext(r.Context())
		if !ok {
			writeError(w, fmt.Errorf("utilisateur non authentifié: %w", ErrUnauthorized))
			return
		}
		var s Service
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			writeError(w, fmt.Errorf("corps JSON invalide: %w", ErrValidation))
			return
		}
		s.ProviderID = providerID
		created, err := createService(r.Context(), db, s)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	}
}

func getServiceHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		s, err := fetchService(r.Context(), db, id)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)
	}
}

func updateServiceHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		providerID, ok := GetUserIDFromContext(r.Context())
		if !ok {
			writeError(w, fmt.Errorf("utilisateur non authentifié: %w", ErrUnauthorized))
			return
		}
		var s Service
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			writeError(w, fmt.Errorf("corps JSON invalide: %w", ErrValidation))
			return
		}
		s.ID = id
		s.ProviderID = providerID
		updated, err := updateService(r.Context(), db, s)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updated)
	}
}

func deleteServiceHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		providerID, ok := GetUserIDFromContext(r.Context())
		if !ok {
			writeError(w, fmt.Errorf("utilisateur non authentifié: %w", ErrUnauthorized))
			return
		}
		if err := deleteService(r.Context(), db, id, providerID); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func listServicesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		services, err := listServiceRows(r.Context(), db, q.Get("categorie"), q.Get("ville"), q.Get("search"))
		if err != nil {
			writeError(w, err)
			return
		}
		if services == nil {
			services = []Service{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(services)
	}
}

// RegisterServiceRoutes enregistre les routes CRUD des services et les stats utilisateur.
func RegisterServiceRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("GET /api/services", listServicesHandler(db))
	mux.HandleFunc("POST /api/services", createServiceHandler(db))
	mux.HandleFunc("GET /api/services/{id}", getServiceHandler(db))
	mux.HandleFunc("PUT /api/services/{id}", updateServiceHandler(db))
	mux.HandleFunc("DELETE /api/services/{id}", deleteServiceHandler(db))
	mux.HandleFunc("GET /api/users/{id}/stats", getUserStatsHandler(db))
}

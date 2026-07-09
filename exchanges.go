package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ValidateAccept vérifie que l'acteur est le propriétaire du service et que l'échange est pending.
func (e Exchange) ValidateAccept(actorID int) error {
	if e.Status != "pending" {
		return fmt.Errorf("transition invalide depuis %q: %w", e.Status, ErrValidation)
	}
	if e.OwnerID != actorID {
		return fmt.Errorf("seul le propriétaire du service peut accepter: %w", ErrValidation)
	}
	return nil
}

// ValidateReject vérifie que l'acteur est le propriétaire et que l'échange est pending.
func (e Exchange) ValidateReject(actorID int) error {
	if e.Status != "pending" {
		return fmt.Errorf("transition invalide depuis %q: %w", e.Status, ErrValidation)
	}
	if e.OwnerID != actorID {
		return fmt.Errorf("seul le propriétaire du service peut refuser: %w", ErrValidation)
	}
	return nil
}

// ValidateComplete vérifie que l'acteur est le demandeur et que l'échange est accepted.
func (e Exchange) ValidateComplete(actorID int) error {
	if e.Status != "accepted" {
		return fmt.Errorf("transition invalide depuis %q: %w", e.Status, ErrValidation)
	}
	if e.RequesterID != actorID {
		return fmt.Errorf("seul le demandeur peut confirmer la prestation rendue: %w", ErrValidation)
	}
	return nil
}

// ValidateCancel vérifie que l'acteur est une des deux parties et que l'échange n'est pas terminal.
func (e Exchange) ValidateCancel(actorID int) error {
	if e.Status != "pending" && e.Status != "accepted" {
		return fmt.Errorf("transition invalide depuis %q: %w", e.Status, ErrValidation)
	}
	if e.RequesterID != actorID && e.OwnerID != actorID {
		return fmt.Errorf("seul le demandeur ou l'offreur peut annuler: %w", ErrValidation)
	}
	return nil
}

// --- Couche métier ---

func validateNewExchange(serviceProviderID, requesterID, serviceCredits, balance int, hasActive bool) error {
	if serviceProviderID == requesterID {
		return fmt.Errorf("impossible de s'échanger son propre service: %w", ErrValidation)
	}
	if hasActive {
		return fmt.Errorf("service déjà réservé: %w", ErrConflict)
	}
	if balance < serviceCredits {
		return fmt.Errorf("crédits insuffisants: %w", ErrValidation)
	}
	return nil
}

func createExchange(ctx context.Context, db *sql.DB, requesterID, serviceID int) (*Exchange, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	service, err := fetchService(ctx, tx, serviceID)
	if err != nil {
		return nil, err
	}
	balance, err := fetchUserBalance(ctx, tx, requesterID)
	if err != nil {
		return nil, err
	}
	hasActive, err := hasActiveExchangeForService(ctx, tx, serviceID)
	if err != nil {
		return nil, err
	}
	if err := validateNewExchange(service.ProviderID, requesterID, service.Credits, balance, hasActive); err != nil {
		return nil, err
	}
	e, err := insertExchangeRow(ctx, tx, serviceID, requesterID, service.ProviderID)
	if err != nil {
		return nil, err
	}
	return e, tx.Commit()
}

func acceptExchange(ctx context.Context, db *sql.DB, exchangeID, actorID int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	e, err := selectExchangeForUpdate(ctx, tx, exchangeID)
	if err != nil {
		return err
	}
	if err := e.ValidateAccept(actorID); err != nil {
		return err
	}
	service, err := fetchService(ctx, tx, e.ServiceID)
	if err != nil {
		return err
	}
	if err := RecordCreditTransaction(ctx, tx, e.RequesterID, e.ID, -service.Credits, "spend"); err != nil {
		return err
	}
	if err := updateExchangeStatus(ctx, tx, e.ID, "accepted"); err != nil {
		return err
	}
	return tx.Commit()
}

func rejectExchange(ctx context.Context, db *sql.DB, exchangeID, actorID int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	e, err := selectExchangeForUpdate(ctx, tx, exchangeID)
	if err != nil {
		return err
	}
	if err := e.ValidateReject(actorID); err != nil {
		return err
	}
	if err := updateExchangeStatus(ctx, tx, e.ID, "rejected"); err != nil {
		return err
	}
	return tx.Commit()
}

func completeExchange(ctx context.Context, db *sql.DB, exchangeID, actorID int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	e, err := selectExchangeForUpdate(ctx, tx, exchangeID)
	if err != nil {
		return err
	}
	if err := e.ValidateComplete(actorID); err != nil {
		return err
	}
	service, err := fetchService(ctx, tx, e.ServiceID)
	if err != nil {
		return err
	}
	if err := RecordCreditTransaction(ctx, tx, e.OwnerID, e.ID, service.Credits, "earn"); err != nil {
		return err
	}
	if err := updateExchangeStatus(ctx, tx, e.ID, "completed"); err != nil {
		return err
	}
	return tx.Commit()
}

func cancelExchange(ctx context.Context, db *sql.DB, exchangeID, actorID int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	e, err := selectExchangeForUpdate(ctx, tx, exchangeID)
	if err != nil {
		return err
	}
	if err := e.ValidateCancel(actorID); err != nil {
		return err
	}
	if e.Status == "accepted" {
		service, err := fetchService(ctx, tx, e.ServiceID)
		if err != nil {
			return err
		}
		if err := RecordCreditTransaction(ctx, tx, e.RequesterID, e.ID, service.Credits, "refund"); err != nil {
			return err
		}
	}
	if err := updateExchangeStatus(ctx, tx, e.ID, "cancelled"); err != nil {
		return err
	}
	return tx.Commit()
}

// --- Handlers HTTP ---

func createExchangeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := GetUserIDFromContext(r.Context())
		if !ok {
			writeError(w, fmt.Errorf("utilisateur non authentifié: %w", ErrUnauthorized))
			return
		}
		var body struct {
			ServiceID int `json:"service_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, fmt.Errorf("corps JSON invalide: %w", ErrValidation))
			return
		}
		e, err := createExchange(r.Context(), db, userID, body.ServiceID)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(e)
	}
}

func getExchangeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		e, err := selectExchange(r.Context(), db, id)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(e)
	}
}

func listExchangesHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := GetUserIDFromContext(r.Context())
		if !ok {
			writeError(w, fmt.Errorf("utilisateur non authentifié: %w", ErrUnauthorized))
			return
		}
		exchanges, err := selectExchangesForUser(r.Context(), db, userID, r.URL.Query().Get("status"))
		if err != nil {
			writeError(w, err)
			return
		}
		if exchanges == nil {
			exchanges = []Exchange{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exchanges)
	}
}

func acceptExchangeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := GetUserIDFromContext(r.Context())
		if !ok {
			writeError(w, fmt.Errorf("utilisateur non authentifié: %w", ErrUnauthorized))
			return
		}
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		if err := acceptExchange(r.Context(), db, id, userID); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func rejectExchangeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := GetUserIDFromContext(r.Context())
		if !ok {
			writeError(w, fmt.Errorf("utilisateur non authentifié: %w", ErrUnauthorized))
			return
		}
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		if err := rejectExchange(r.Context(), db, id, userID); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func completeExchangeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := GetUserIDFromContext(r.Context())
		if !ok {
			writeError(w, fmt.Errorf("utilisateur non authentifié: %w", ErrUnauthorized))
			return
		}
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		if err := completeExchange(r.Context(), db, id, userID); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func cancelExchangeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := GetUserIDFromContext(r.Context())
		if !ok {
			writeError(w, fmt.Errorf("utilisateur non authentifié: %w", ErrUnauthorized))
			return
		}
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		if err := cancelExchange(r.Context(), db, id, userID); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// RegisterExchangeRoutes enregistre les 7 routes du système d'échange sur le mux fourni.
func RegisterExchangeRoutes(mux *http.ServeMux, db *sql.DB) {
	mux.HandleFunc("POST /api/exchanges", createExchangeHandler(db))
	mux.HandleFunc("GET /api/exchanges", listExchangesHandler(db))
	mux.HandleFunc("GET /api/exchanges/{id}", getExchangeHandler(db))
	mux.HandleFunc("PUT /api/exchanges/{id}/accept", acceptExchangeHandler(db))
	mux.HandleFunc("PUT /api/exchanges/{id}/reject", rejectExchangeHandler(db))
	mux.HandleFunc("PUT /api/exchanges/{id}/complete", completeExchangeHandler(db))
	mux.HandleFunc("PUT /api/exchanges/{id}/cancel", cancelExchangeHandler(db))
}
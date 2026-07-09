package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func validateReview(status string, note int) error {
	if status != "completed" {
		return fmt.Errorf("échange non terminé: %w", ErrValidation)
	}
	if note < 1 || note > 5 {
		return fmt.Errorf("note hors bornes: %w", ErrValidation)
	}
	return nil
}

func createReview(ctx context.Context, db *sql.DB, r Review) (*Review, error) {
	requesterID, ownerID, status, err := selectExchangeParties(ctx, db, r.ExchangeID)
	if err != nil {
		return nil, err
	}
	// La cible est déduite côté serveur : l'auteur doit être un participant de l'échange,
	// et la cible est forcément l'autre partie. On ne fait jamais confiance à un target_id
	// fourni par le client.
	switch r.AuthorID {
	case requesterID:
		r.TargetID = ownerID
	case ownerID:
		r.TargetID = requesterID
	default:
		return nil, fmt.Errorf("auteur non participant de l'échange %d: %w", r.ExchangeID, ErrUnauthorized)
	}
	if err := validateReview(status, r.Note); err != nil {
		return nil, err
	}
	return insertReview(ctx, db, r)
}

func createReviewHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exchangeID, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		authorID, ok := GetUserIDFromContext(r.Context())
		if !ok {
			writeError(w, fmt.Errorf("utilisateur non authentifié: %w", ErrUnauthorized))
			return
		}
		var body struct {
			Note        int    `json:"note"`
			Commentaire string `json:"commentaire"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, fmt.Errorf("corps JSON invalide: %w", ErrValidation))
			return
		}
		rv := Review{
			ExchangeID:  exchangeID,
			AuthorID:    authorID,
			Note:        body.Note,
			Commentaire: body.Commentaire,
		}
		created, err := createReview(r.Context(), db, rv)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)
	}
}

func getUserReviewsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		reviews, err := selectUserReviews(r.Context(), db, id)
		if err != nil {
			writeError(w, err)
			return
		}
		if reviews == nil {
			reviews = []Review{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reviews)
	}
}

func getServiceReviewsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		reviews, err := selectServiceReviews(r.Context(), db, id)
		if err != nil {
			writeError(w, err)
			return
		}
		if reviews == nil {
			reviews = []Review{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reviews)
	}
}

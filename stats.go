package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func getUserStats(ctx context.Context, db *sql.DB, userID int) (*UserStats, error) {
	if _, err := selectUser(ctx, db, userID); err != nil {
		return nil, err
	}
	s := UserStats{UserID: userID}
	err := db.QueryRowContext(ctx, `
        SELECT
            (SELECT COUNT(*) FROM services WHERE provider_id = $1 AND actif = true),
            (SELECT COUNT(*) FROM exchanges WHERE (requester_id = $1 OR owner_id = $1) AND status = 'completed'),
            (SELECT credit_balance FROM users WHERE id = $1),
            COALESCE((SELECT AVG(note) FROM reviews WHERE target_id = $1), 0),
            (SELECT COUNT(*) FROM reviews WHERE target_id = $1),
            COALESCE((SELECT SUM(montant) FROM credit_transactions WHERE user_id = $1 AND type = 'earn'), 0),
            COALESCE((SELECT -SUM(montant) FROM credit_transactions WHERE user_id = $1 AND type = 'spend'), 0)
    `, userID).Scan(&s.ServicesActifs, &s.EchangesCompletes, &s.CreditBalance,
		&s.NoteMoyenne, &s.NbAvis, &s.TotalGagne, &s.TotalDepense)
	return &s, err
}

func getUserStatsHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("id"))
		if err != nil {
			writeError(w, fmt.Errorf("id invalide: %w", ErrValidation))
			return
		}
		stats, err := getUserStats(r.Context(), db, id)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

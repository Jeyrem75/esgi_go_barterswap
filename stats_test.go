package main

import (
	"context"
	"testing"
)

func TestGetUserStats(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	u, _ := insertUser(ctx, db, User{Pseudo: "stats-full"})

	stats, err := getUserStats(ctx, db, u.ID)
	if err != nil {
		t.Fatalf("getUserStats: %v", err)
	}
	if stats.CreditBalance != 10 {
		t.Errorf("credit_balance = %d, want 10", stats.CreditBalance)
	}
	if stats.ServicesActifs != 0 {
		t.Errorf("services_actifs = %d, want 0", stats.ServicesActifs)
	}
	if stats.EchangesCompletes != 0 {
		t.Errorf("echanges_completes = %d, want 0", stats.EchangesCompletes)
	}
	if stats.NoteMoyenne != 0 {
		t.Errorf("note_moyenne = %f, want 0", stats.NoteMoyenne)
	}

	// Vérifie que CreditBalance dans stats == CreditBalance dans GET /users/{id}
	userRow, _ := selectUser(ctx, db, u.ID)
	if stats.CreditBalance != userRow.CreditBalance {
		t.Errorf("incohérence stats vs user: %d != %d", stats.CreditBalance, userRow.CreditBalance)
	}
}

package main

import (
	"context"
	"errors"
	"testing"
)

func TestRecordCreditTransaction(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	owner, _ := insertUser(ctx, db, User{Pseudo: "owner-credit"})
	requester, _ := insertUser(ctx, db, User{Pseudo: "requester-credit"})

	// Un échange est requis (clé étrangère exchange_id sur credit_transactions).
	var serviceID int
	db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville) VALUES ($1,'Cours','','Cuisine',60,5,'Paris') RETURNING id`,
		owner.ID).Scan(&serviceID)
	var exchangeID int
	db.QueryRowContext(ctx,
		`INSERT INTO exchanges (service_id, requester_id, owner_id, status) VALUES ($1,$2,$3,'accepted') RETURNING id`,
		serviceID, requester.ID, owner.ID).Scan(&exchangeID)

	t.Run("débit valide (spend)", func(t *testing.T) {
		tx, _ := db.BeginTx(ctx, nil)
		defer tx.Rollback()
		if err := RecordCreditTransaction(ctx, tx, requester.ID, exchangeID, -5, "spend"); err != nil {
			t.Fatalf("RecordCreditTransaction: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		if bal, _ := fetchUserBalance(ctx, db, requester.ID); bal != 5 {
			t.Errorf("solde = %d, want 5", bal)
		}
	})

	t.Run("crédit valide (earn)", func(t *testing.T) {
		tx, _ := db.BeginTx(ctx, nil)
		defer tx.Rollback()
		if err := RecordCreditTransaction(ctx, tx, owner.ID, exchangeID, 5, "earn"); err != nil {
			t.Fatalf("RecordCreditTransaction: %v", err)
		}
		if err := tx.Commit(); err != nil {
			t.Fatalf("commit: %v", err)
		}
		if bal, _ := fetchUserBalance(ctx, db, owner.ID); bal != 15 {
			t.Errorf("solde = %d, want 15", bal)
		}
	})

	t.Run("solde insuffisant → ErrValidation, rollback", func(t *testing.T) {
		tx, _ := db.BeginTx(ctx, nil)
		defer tx.Rollback()
		// requester est à 5 crédits ; débiter 999 doit échouer.
		err := RecordCreditTransaction(ctx, tx, requester.ID, exchangeID, -999, "spend")
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("attendu ErrValidation, got %v", err)
		}
		// Le solde ne doit pas avoir bougé (transaction avortée).
		tx.Rollback()
		if bal, _ := fetchUserBalance(ctx, db, requester.ID); bal != 5 {
			t.Errorf("solde après échec = %d, want 5 (inchangé)", bal)
		}
	})
}

// TestWelcomeCreditIsJournaled vérifie que le bonus de bienvenue passe par le journal de transactions.
func TestWelcomeCreditIsJournaled(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	u, err := insertUser(ctx, db, User{Pseudo: "welcome-journal"})
	if err != nil {
		t.Fatalf("insertUser: %v", err)
	}

	var count, montant int
	var txType string
	err = db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(montant), 0), MAX(type)
         FROM credit_transactions WHERE user_id = $1 AND exchange_id IS NULL`,
		u.ID).Scan(&count, &montant, &txType)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Fatalf("nb transactions bonus de bienvenue = %d, want 1", count)
	}
	if montant != 10 {
		t.Errorf("montant = %d, want 10", montant)
	}
	if txType != "earn" {
		t.Errorf("type = %q, want earn", txType)
	}
}

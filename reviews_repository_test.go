package main

import (
	"context"
	"errors"
	"testing"
)

func TestInsertReview(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	// Fixtures : deux users, un service, un échange completed
	owner, _ := insertUser(ctx, db, User{Pseudo: "owner-rev"})
	requester, _ := insertUser(ctx, db, User{Pseudo: "requester-rev"})

	// Insère skill + service + échange directement en SQL
	db.ExecContext(ctx, `INSERT INTO skills (user_id, nom, niveau) VALUES ($1,'Cuisine','expert')`, owner.ID)
	var serviceID int
	db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, categorie, duree_minutes, credits) VALUES ($1,'Cours cuisine','Cuisine',60,5) RETURNING id`,
		owner.ID).Scan(&serviceID)
	var exchangeID int
	db.QueryRowContext(ctx,
		`INSERT INTO exchanges (service_id, requester_id, owner_id, status) VALUES ($1,$2,$3,'completed') RETURNING id`,
		serviceID, requester.ID, owner.ID).Scan(&exchangeID)

	t.Run("insertion OK", func(t *testing.T) {
		rv, err := insertReview(ctx, db, Review{
			ExchangeID: exchangeID,
			AuthorID:   requester.ID,
			TargetID:   owner.ID,
			Note:       4,
		})
		if err != nil {
			t.Fatalf("insertReview: %v", err)
		}
		if rv.ID == 0 {
			t.Error("ID attendu non nul")
		}
	})

	t.Run("deuxième avis même échange → 400 (test #11)", func(t *testing.T) {
		_, err := insertReview(ctx, db, Review{
			ExchangeID: exchangeID,
			AuthorID:   requester.ID,
			TargetID:   owner.ID,
			Note:       5,
		})
		if !errors.Is(err, ErrValidation) {
			t.Errorf("attendu ErrValidation, got %v", err)
		}
	})
}

func TestSelectUserAndServiceReviews(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	reviews, err := selectUserReviews(ctx, db, 999999)
	if err != nil {
		t.Fatalf("selectUserReviews: %v", err)
	}
	if reviews != nil {
		t.Error("attendu nil pour user inexistant")
	}

	serviceReviews, err := selectServiceReviews(ctx, db, 999999)
	if err != nil {
		t.Fatalf("selectServiceReviews: %v", err)
	}
	if serviceReviews != nil {
		t.Error("attendu nil pour service inexistant")
	}
}

package main

import (
	"context"
	"errors"
	"testing"
)

// TestExchangeRepository couvre insertion, lecture, liste filtrée et mise à jour
// de statut de la couche repository. Nécessite DATABASE_URL.
func TestExchangeRepository(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	owner, _ := insertUser(ctx, db, User{Pseudo: "owner-repo"})
	requester, _ := insertUser(ctx, db, User{Pseudo: "requester-repo"})

	var serviceID int
	db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville) VALUES ($1,'Cours','','Cuisine',60,5,'Paris') RETURNING id`,
		owner.ID).Scan(&serviceID)

	// insertExchangeRow — la ligne démarre en pending.
	e, err := insertExchangeRow(ctx, db, serviceID, requester.ID, owner.ID)
	if err != nil {
		t.Fatalf("insertExchangeRow: %v", err)
	}
	if e.ID == 0 || e.Status != "pending" {
		t.Fatalf("échange inattendu: %+v", e)
	}

	// selectExchange — relit la même ligne.
	got, err := selectExchange(ctx, db, e.ID)
	if err != nil {
		t.Fatalf("selectExchange: %v", err)
	}
	if got.ServiceID != serviceID || got.RequesterID != requester.ID || got.OwnerID != owner.ID {
		t.Errorf("participants incohérents: %+v", got)
	}

	// selectExchange sur id inexistant → ErrNotFound.
	if _, err := selectExchange(ctx, db, 999999); !errors.Is(err, ErrNotFound) {
		t.Errorf("attendu ErrNotFound, got %v", err)
	}

	// updateExchangeStatus + relecture.
	tx, _ := db.BeginTx(ctx, nil)
	if err := updateExchangeStatus(ctx, tx, e.ID, "accepted"); err != nil {
		t.Fatalf("updateExchangeStatus: %v", err)
	}
	tx.Commit()
	got, _ = selectExchange(ctx, db, e.ID)
	if got.Status != "accepted" {
		t.Errorf("status = %q, want accepted", got.Status)
	}

	// selectExchangesForUser — l'échange apparaît pour les deux participants.
	list, err := selectExchangesForUser(ctx, db, requester.ID, "")
	if err != nil {
		t.Fatalf("selectExchangesForUser: %v", err)
	}
	if !containsExchange(list, e.ID) {
		t.Errorf("échange %d absent de la liste du demandeur", e.ID)
	}

	// Filtre par statut : "pending" ne doit pas contenir notre échange (désormais accepted).
	pendings, _ := selectExchangesForUser(ctx, db, requester.ID, "pending")
	if containsExchange(pendings, e.ID) {
		t.Errorf("échange accepted %d ne devrait pas apparaître dans le filtre pending", e.ID)
	}
}

func TestHasActiveExchangeForService(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	owner, _ := insertUser(ctx, db, User{Pseudo: "owner-active"})
	requester, _ := insertUser(ctx, db, User{Pseudo: "requester-active"})

	var serviceID int
	db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville) VALUES ($1,'Cours','','Cuisine',60,5,'Paris') RETURNING id`,
		owner.ID).Scan(&serviceID)

	// Aucun échange → false.
	active, err := hasActiveExchangeForService(ctx, db, serviceID)
	if err != nil {
		t.Fatalf("hasActiveExchangeForService: %v", err)
	}
	if active {
		t.Error("attendu false sans échange")
	}

	// Un échange pending → true.
	if _, err := insertExchangeRow(ctx, db, serviceID, requester.ID, owner.ID); err != nil {
		t.Fatalf("insertExchangeRow: %v", err)
	}
	active, _ = hasActiveExchangeForService(ctx, db, serviceID)
	if !active {
		t.Error("attendu true avec un échange pending")
	}
}

func containsExchange(list []Exchange, id int) bool {
	for _, e := range list {
		if e.ID == id {
			return true
		}
	}
	return false
}

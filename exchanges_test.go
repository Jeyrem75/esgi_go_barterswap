package main

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// --- Tests unitaires de la machine à états (fonctions pures, sans DB) ---

func TestValidateAccept(t *testing.T) {
	tests := []struct {
		name    string
		e       Exchange
		actorID int
		wantErr error
	}{
		{"owner accepte un pending", Exchange{Status: "pending", OwnerID: 1, RequesterID: 2}, 1, nil},
		{"non-owner ne peut pas accepter", Exchange{Status: "pending", OwnerID: 1, RequesterID: 2}, 2, ErrValidation},
		{"déjà accepted", Exchange{Status: "accepted", OwnerID: 1, RequesterID: 2}, 1, ErrValidation},
		{"déjà completed", Exchange{Status: "completed", OwnerID: 1, RequesterID: 2}, 1, ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.e.ValidateAccept(tt.actorID)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("attendu nil, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("attendu %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateReject(t *testing.T) {
	tests := []struct {
		name    string
		e       Exchange
		actorID int
		wantErr error
	}{
		{"owner refuse un pending", Exchange{Status: "pending", OwnerID: 1, RequesterID: 2}, 1, nil},
		{"non-owner ne peut pas refuser", Exchange{Status: "pending", OwnerID: 1, RequesterID: 2}, 2, ErrValidation},
		{"refus sur accepted interdit", Exchange{Status: "accepted", OwnerID: 1, RequesterID: 2}, 1, ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.e.ValidateReject(tt.actorID)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("attendu nil, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("attendu %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateComplete(t *testing.T) {
	tests := []struct {
		name    string
		e       Exchange
		actorID int
		wantErr error
	}{
		{"demandeur confirme un accepted", Exchange{Status: "accepted", OwnerID: 1, RequesterID: 2}, 2, nil},
		{"owner ne peut pas confirmer", Exchange{Status: "accepted", OwnerID: 1, RequesterID: 2}, 1, ErrValidation},
		{"completion sur pending interdite", Exchange{Status: "pending", OwnerID: 1, RequesterID: 2}, 2, ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.e.ValidateComplete(tt.actorID)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("attendu nil, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("attendu %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateCancel(t *testing.T) {
	tests := []struct {
		name    string
		e       Exchange
		actorID int
		wantErr error
	}{
		{"demandeur annule un pending", Exchange{Status: "pending", OwnerID: 1, RequesterID: 2}, 2, nil},
		{"owner annule un accepted", Exchange{Status: "accepted", OwnerID: 1, RequesterID: 2}, 1, nil},
		{"tiers ne peut pas annuler", Exchange{Status: "pending", OwnerID: 1, RequesterID: 2}, 99, ErrValidation},
		{"annulation d'un completed interdite", Exchange{Status: "completed", OwnerID: 1, RequesterID: 2}, 2, ErrValidation},
		{"annulation d'un rejected interdite", Exchange{Status: "rejected", OwnerID: 1, RequesterID: 2}, 2, ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.e.ValidateCancel(tt.actorID)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("attendu nil, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("attendu %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateNewExchange(t *testing.T) {
	tests := []struct {
		name       string
		providerID int
		requester  int
		credits    int
		balance    int
		hasActive  bool
		wantErr    error
	}{
		{"échange valide", 1, 2, 5, 10, false, nil},
		{"solde exactement suffisant", 1, 2, 10, 10, false, nil},
		{"son propre service", 1, 1, 5, 10, false, ErrValidation},
		{"service déjà réservé", 1, 2, 5, 10, true, ErrConflict},
		{"crédits insuffisants", 1, 2, 15, 10, false, ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNewExchange(tt.providerID, tt.requester, tt.credits, tt.balance, tt.hasActive)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("attendu nil, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("attendu %v, got %v", tt.wantErr, err)
			}
		})
	}
}

// --- Tests d'intégration du cycle de vie complet (nécessitent DATABASE_URL) ---

func TestExchangeLifecycle(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	owner, _ := insertUser(ctx, db, User{Pseudo: "owner-exch"})
	requester, _ := insertUser(ctx, db, User{Pseudo: "requester-exch"})

	var serviceID int
	db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville) VALUES ($1,'Cours','','Cuisine',60,5,'Paris') RETURNING id`,
		owner.ID).Scan(&serviceID)

	// Création : l'échange démarre en pending, sans mouvement de crédit.
	e, err := createExchange(ctx, db, requester.ID, serviceID)
	if err != nil {
		t.Fatalf("createExchange: %v", err)
	}
	if e.Status != "pending" {
		t.Fatalf("status = %q, want pending", e.Status)
	}

	// Acceptation par l'owner : le demandeur est débité de 5 crédits.
	if err := acceptExchange(ctx, db, e.ID, owner.ID); err != nil {
		t.Fatalf("acceptExchange: %v", err)
	}
	if bal, _ := fetchUserBalance(ctx, db, requester.ID); bal != 5 {
		t.Errorf("solde demandeur après accept = %d, want 5", bal)
	}

	// Complétion par le demandeur : l'owner est crédité de 5.
	if err := completeExchange(ctx, db, e.ID, requester.ID); err != nil {
		t.Fatalf("completeExchange: %v", err)
	}
	if bal, _ := fetchUserBalance(ctx, db, owner.ID); bal != 15 {
		t.Errorf("solde owner après complete = %d, want 15", bal)
	}
}

func TestExchangeCancelRefund(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	owner, _ := insertUser(ctx, db, User{Pseudo: "owner-cancel"})
	requester, _ := insertUser(ctx, db, User{Pseudo: "requester-cancel"})

	var serviceID int
	db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville) VALUES ($1,'Cours','','Cuisine',60,5,'Paris') RETURNING id`,
		owner.ID).Scan(&serviceID)

	e, err := createExchange(ctx, db, requester.ID, serviceID)
	if err != nil {
		t.Fatalf("createExchange: %v", err)
	}
	if err := acceptExchange(ctx, db, e.ID, owner.ID); err != nil {
		t.Fatalf("acceptExchange: %v", err)
	}
	// Après accept le demandeur est à 5. L'annulation d'un accepted rembourse.
	if err := cancelExchange(ctx, db, e.ID, requester.ID); err != nil {
		t.Fatalf("cancelExchange: %v", err)
	}
	if bal, _ := fetchUserBalance(ctx, db, requester.ID); bal != 10 {
		t.Errorf("solde demandeur après remboursement = %d, want 10", bal)
	}
}

// TestConcurrentAccept vérifie que deux acceptations simultanées du même échange
// ne réussissent qu'une seule fois : le verrou FOR UPDATE de selectExchangeForUpdate
// sérialise les deux transactions, la seconde voyant alors un statut déjà "accepted".
// Exigé par la fiche dev-C (ticket C4 / DoD "test de concurrence").
func TestConcurrentAccept(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	owner, _ := insertUser(ctx, db, User{Pseudo: "owner-concurrent"})
	requester, _ := insertUser(ctx, db, User{Pseudo: "requester-concurrent"})

	var serviceID int
	db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville) VALUES ($1,'Cours','','Cuisine',60,5,'Paris') RETURNING id`,
		owner.ID).Scan(&serviceID)

	e, err := createExchange(ctx, db, requester.ID, serviceID)
	if err != nil {
		t.Fatalf("createExchange: %v", err)
	}

	const n = 2
	var wg sync.WaitGroup
	errs := make(chan error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // relâche les goroutines en même temps
			errs <- acceptExchange(ctx, db, e.ID, owner.ID)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	var success int
	for err := range errs {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Errorf("accepts réussis = %d, want 1 (le FOR UPDATE doit sérialiser)", success)
	}

	// Le demandeur ne doit avoir été débité qu'une seule fois (5 crédits sur 10).
	if bal, _ := fetchUserBalance(ctx, db, requester.ID); bal != 5 {
		t.Errorf("solde demandeur = %d, want 5 (un seul débit)", bal)
	}
}

// TestConcurrentCreateExchange vérifie que deux demandes simultanées sur le même service n'en font passer qu'une (409 sur l'autre).
func TestConcurrentCreateExchange(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	owner, _ := insertUser(ctx, db, User{Pseudo: "owner-create-race"})
	r1, _ := insertUser(ctx, db, User{Pseudo: "requester-race-1"})
	r2, _ := insertUser(ctx, db, User{Pseudo: "requester-race-2"})

	var serviceID int
	db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville) VALUES ($1,'Cours','','Cuisine',60,5,'Paris') RETURNING id`,
		owner.ID).Scan(&serviceID)

	requesters := []int{r1.ID, r2.ID}
	const n = 2
	var wg sync.WaitGroup
	errs := make(chan error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		reqID := requesters[i]
		go func() {
			defer wg.Done()
			<-start // relâche les goroutines en même temps
			_, err := createExchange(ctx, db, reqID, serviceID)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	var success, conflicts int
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrConflict):
			conflicts++
		default:
			t.Errorf("erreur inattendue: %v", err)
		}
	}
	if success != 1 {
		t.Errorf("créations réussies = %d, want 1 (le FOR UPDATE doit sérialiser)", success)
	}
	if conflicts != 1 {
		t.Errorf("conflits = %d, want 1", conflicts)
	}
}

func TestExchangeReject(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	owner, _ := insertUser(ctx, db, User{Pseudo: "owner-reject"})
	requester, _ := insertUser(ctx, db, User{Pseudo: "requester-reject"})

	var serviceID int
	db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville) VALUES ($1,'Cours','','Cuisine',60,5,'Paris') RETURNING id`,
		owner.ID).Scan(&serviceID)

	e, err := createExchange(ctx, db, requester.ID, serviceID)
	if err != nil {
		t.Fatalf("createExchange: %v", err)
	}
	if err := rejectExchange(ctx, db, e.ID, owner.ID); err != nil {
		t.Fatalf("rejectExchange: %v", err)
	}
	got, _ := selectExchange(ctx, db, e.ID)
	if got.Status != "rejected" {
		t.Errorf("status = %q, want rejected", got.Status)
	}
	// Un refus ne débite personne.
	if bal, _ := fetchUserBalance(ctx, db, requester.ID); bal != 10 {
		t.Errorf("solde demandeur après refus = %d, want 10", bal)
	}
}

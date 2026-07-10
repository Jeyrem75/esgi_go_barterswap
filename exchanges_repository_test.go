package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)


func TestExchangeRepository(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	owner, _ := insertUser(ctx, db, User{Pseudo: "owner-repo"})
	requester, _ := insertUser(ctx, db, User{Pseudo: "requester-repo"})

	var serviceID int
	db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville) VALUES ($1,'Cours','','Cuisine',60,5,'Paris') RETURNING id`,
		owner.ID).Scan(&serviceID)

	
	e, err := insertExchangeRow(ctx, db, serviceID, requester.ID, owner.ID)
	if err != nil {
		t.Fatalf("insertExchangeRow: %v", err)
	}
	if e.ID == 0 || e.Status != "pending" {
		t.Fatalf("échange inattendu: %+v", e)
	}

	
	got, err := selectExchange(ctx, db, e.ID)
	if err != nil {
		t.Fatalf("selectExchange: %v", err)
	}
	if got.ServiceID != serviceID || got.RequesterID != requester.ID || got.OwnerID != owner.ID {
		t.Errorf("participants incohérents: %+v", got)
	}

	
	if _, err := selectExchange(ctx, db, 999999); !errors.Is(err, ErrNotFound) {
		t.Errorf("attendu ErrNotFound, got %v", err)
	}

	
	tx, _ := db.BeginTx(ctx, nil)
	if err := updateExchangeStatus(ctx, tx, e.ID, "accepted"); err != nil {
		t.Fatalf("updateExchangeStatus: %v", err)
	}
	tx.Commit()
	got, _ = selectExchange(ctx, db, e.ID)
	if got.Status != "accepted" {
		t.Errorf("status = %q, want accepted", got.Status)
	}

	
	list, err := selectExchangesForUser(ctx, db, requester.ID, "")
	if err != nil {
		t.Fatalf("selectExchangesForUser: %v", err)
	}
	if !containsExchange(list, e.ID) {
		t.Errorf("échange %d absent de la liste du demandeur", e.ID)
	}

	
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

	
	active, err := hasActiveExchangeForService(ctx, db, serviceID)
	if err != nil {
		t.Fatalf("hasActiveExchangeForService: %v", err)
	}
	if active {
		t.Error("attendu false sans échange")
	}

	
	if _, err := insertExchangeRow(ctx, db, serviceID, requester.ID, owner.ID); err != nil {
		t.Fatalf("insertExchangeRow: %v", err)
	}
	active, _ = hasActiveExchangeForService(ctx, db, serviceID)
	if !active {
		t.Error("attendu true avec un échange pending")
	}
}


func TestExchangeLifecycleHTTP(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	mux := http.NewServeMux()
	RegisterUserRoutes(mux, db)
	RegisterExchangeRoutes(mux, db)

	owner, _ := insertUser(ctx, db, User{Pseudo: "owner-lifecycle-http"})
	requester, _ := insertUser(ctx, db, User{Pseudo: "requester-lifecycle-http"})

	var serviceID int
	db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville) VALUES ($1,'Cours cuisine','','Cuisine',60,5,'Paris') RETURNING id`,
		owner.ID).Scan(&serviceID)

	doReq := func(method, path, body string, userID int) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", strconv.Itoa(userID))
		w := httptest.NewRecorder()
		AuthMiddleware(mux).ServeHTTP(w, req)
		return w
	}

	var exchangeID int

	t.Run("POST /api/exchanges → 201, pending", func(t *testing.T) {
		body := `{"service_id":` + strconv.Itoa(serviceID) + `}`
		w := doReq("POST", "/api/exchanges", body, requester.ID)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201 — %s", w.Code, w.Body.String())
		}
		var e Exchange
		json.NewDecoder(w.Body).Decode(&e)
		if e.Status != "pending" {
			t.Errorf("status = %q, want pending", e.Status)
		}
		exchangeID = e.ID
	})

	t.Run("GET /api/exchanges?status=pending → contient l'échange", func(t *testing.T) {
		w := doReq("GET", "/api/exchanges?status=pending", "", requester.ID)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 — %s", w.Code, w.Body.String())
		}
		var list []Exchange
		json.NewDecoder(w.Body).Decode(&list)
		if !containsExchange(list, exchangeID) {
			t.Error("échange absent de la liste filtrée par status=pending")
		}
	})

	t.Run("PUT /api/exchanges/{id}/accept → 200, crédits bloqués (test #7)", func(t *testing.T) {
		w := doReq("PUT", "/api/exchanges/"+strconv.Itoa(exchangeID)+"/accept", "", owner.ID)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 — %s", w.Code, w.Body.String())
		}
		if bal, _ := fetchUserBalance(ctx, db, requester.ID); bal != 5 {
			t.Errorf("solde demandeur = %d, want 5 (10 - 5 bloqués)", bal)
		}
		g := doReq("GET", "/api/exchanges/"+strconv.Itoa(exchangeID), "", requester.ID)
		var e Exchange
		json.NewDecoder(g.Body).Decode(&e)
		if e.Status != "accepted" {
			t.Errorf("status = %q, want accepted", e.Status)
		}
	})

	t.Run("PUT /api/exchanges/{id}/complete → 200, crédits transférés (test #8)", func(t *testing.T) {
		w := doReq("PUT", "/api/exchanges/"+strconv.Itoa(exchangeID)+"/complete", "", requester.ID)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 — %s", w.Code, w.Body.String())
		}
		if bal, _ := fetchUserBalance(ctx, db, owner.ID); bal != 15 {
			t.Errorf("solde offreur = %d, want 15 (10 + 5 gagnés)", bal)
		}
		g := doReq("GET", "/api/exchanges/"+strconv.Itoa(exchangeID), "", requester.ID)
		var e Exchange
		json.NewDecoder(g.Body).Decode(&e)
		if e.Status != "completed" {
			t.Errorf("status = %q, want completed", e.Status)
		}
	})
}

func containsExchange(list []Exchange, id int) bool {
	for _, e := range list {
		if e.ID == id {
			return true
		}
	}
	return false
}

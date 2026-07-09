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

func setupServiceFixture(t *testing.T) (providerID int, serviceID int) {
	t.Helper()
	db := testDB(t)
	ctx := context.Background()

	u, err := insertUser(ctx, db, User{Pseudo: "provider-b"})
	if err != nil {
		t.Fatalf("insertUser: %v", err)
	}
	db.ExecContext(ctx, `INSERT INTO skills (user_id, nom, niveau) VALUES ($1,'Informatique','expert')`, u.ID)

	s, err := insertServiceRow(ctx, db, Service{
		ProviderID:   u.ID,
		Titre:        "Cours Go",
		Categorie:    "Informatique",
		DureeMinutes: 60,
		Credits:      5,
	})
	if err != nil {
		t.Fatalf("insertServiceRow: %v", err)
	}
	return u.ID, s.ID
}

func TestInsertServiceRow(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	u, _ := insertUser(ctx, db, User{Pseudo: "svc-insert"})
	s, err := insertServiceRow(ctx, db, Service{
		ProviderID:   u.ID,
		Titre:        "Test service",
		Categorie:    "Jardinage",
		DureeMinutes: 30,
		Credits:      3,
	})
	if err != nil {
		t.Fatalf("insertServiceRow: %v", err)
	}
	if s.ID == 0 {
		t.Error("ID attendu non nul")
	}
	if !s.Actif {
		t.Error("service doit être actif à la création")
	}
}

func TestDeleteServiceRow(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	providerID, serviceID := setupServiceFixture(t)

	t.Run("suppression réelle", func(t *testing.T) {
		n, err := deleteServiceRow(ctx, db, serviceID, providerID)
		if err != nil {
			t.Fatalf("deleteServiceRow: %v", err)
		}
		if n != 1 {
			t.Errorf("rows affected = %d, want 1", n)
		}
	})

	t.Run("0 lignes si mauvais provider", func(t *testing.T) {
		providerID2, serviceID2 := setupServiceFixture(t)
		n, err := deleteServiceRow(ctx, db, serviceID2, providerID2+999)
		if err != nil {
			t.Fatalf("deleteServiceRow: %v", err)
		}
		if n != 0 {
			t.Errorf("rows affected = %d, want 0", n)
		}
	})
}

func TestListServiceRowsFilters(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	u, _ := insertUser(ctx, db, User{Pseudo: "svc-list", Ville: "Lyon"})
	insertServiceRow(ctx, db, Service{ProviderID: u.ID, Titre: "Jardinage Lyon", Categorie: "Jardinage", DureeMinutes: 60, Credits: 2, Ville: "Lyon"})
	insertServiceRow(ctx, db, Service{ProviderID: u.ID, Titre: "Cuisine Paris", Categorie: "Cuisine", DureeMinutes: 60, Credits: 2, Ville: "Paris"})

	t.Run("filtre catégorie", func(t *testing.T) {
		svcs, err := listServiceRows(ctx, db, "Jardinage", "", "")
		if err != nil {
			t.Fatalf("listServiceRows: %v", err)
		}
		for _, s := range svcs {
			if s.Categorie != "Jardinage" {
				t.Errorf("catégorie inattendue: %s", s.Categorie)
			}
		}
	})

	t.Run("filtre search", func(t *testing.T) {
		svcs, err := listServiceRows(ctx, db, "", "", "Lyon")
		if err != nil {
			t.Fatalf("listServiceRows: %v", err)
		}
		if len(svcs) == 0 {
			t.Error("attendu au moins 1 résultat pour search=Lyon")
		}
	})
}

func TestDeleteServiceWithActiveExchange(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	providerID, serviceID := setupServiceFixture(t)
	requester, _ := insertUser(ctx, db, User{Pseudo: "requester-del"})
	db.ExecContext(ctx, `INSERT INTO exchanges (service_id, requester_id, owner_id, status) VALUES ($1,$2,$3,'pending')`,
		serviceID, requester.ID, providerID)

	err := deleteService(ctx, db, serviceID, providerID)
	if !errors.Is(err, ErrConflict) {
		t.Errorf("attendu ErrConflict, got %v", err)
	}
}

// --- Tests httptest bout-en-bout ---

func TestCreateServiceHTTP(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	mux := http.NewServeMux()
	RegisterUserRoutes(mux, db)
	RegisterServiceRoutes(mux, db)

	u, _ := insertUser(ctx, db, User{Pseudo: "provider-http"})
	db.ExecContext(ctx, `INSERT INTO skills (user_id, nom, niveau) VALUES ($1,'Informatique','expert')`, u.ID)

	t.Run("201 service créé", func(t *testing.T) {
		body := `{"titre":"Cours Go","categorie":"Informatique","duree_minutes":60,"credits":5}`
		req := httptest.NewRequest("POST", "/api/services", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", strconv.Itoa(u.ID))
		w := httptest.NewRecorder()
		AuthMiddleware(mux).ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201 — body: %s", w.Code, w.Body.String())
		}
	})

	t.Run("400 sans la compétence (test #3)", func(t *testing.T) {
		body := `{"titre":"Jardinage","categorie":"Jardinage","duree_minutes":60,"credits":3}`
		req := httptest.NewRequest("POST", "/api/services", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", strconv.Itoa(u.ID))
		w := httptest.NewRecorder()
		AuthMiddleware(mux).ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", w.Code)
		}
	})
}

func TestGetAndUpdateServiceHTTP(t *testing.T) {
	db := testDB(t)
	mux := http.NewServeMux()
	RegisterUserRoutes(mux, db)
	RegisterServiceRoutes(mux, db)

	providerID, serviceID := setupServiceFixture(t)

	doReq := func(method, path, body string, userID int) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", strconv.Itoa(userID))
		w := httptest.NewRecorder()
		AuthMiddleware(mux).ServeHTTP(w, req)
		return w
	}

	t.Run("GET /api/services/{id} → 200", func(t *testing.T) {
		w := doReq("GET", "/api/services/"+strconv.Itoa(serviceID), "", providerID)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 — %s", w.Code, w.Body.String())
		}
		var s Service
		json.NewDecoder(w.Body).Decode(&s)
		if s.ID != serviceID {
			t.Errorf("id = %d, want %d", s.ID, serviceID)
		}
	})

	t.Run("GET /api/services?categorie=Informatique → 200", func(t *testing.T) {
		w := doReq("GET", "/api/services?categorie=Informatique", "", providerID)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 — %s", w.Code, w.Body.String())
		}
		var svcs []Service
		json.NewDecoder(w.Body).Decode(&svcs)
		if len(svcs) == 0 {
			t.Error("attendu au moins 1 service Informatique")
		}
	})

	t.Run("PUT /api/services/{id} → 200", func(t *testing.T) {
		body := `{"titre":"Cours Go avancé","categorie":"Informatique","duree_minutes":90,"credits":8}`
		w := doReq("PUT", "/api/services/"+strconv.Itoa(serviceID), body, providerID)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 — %s", w.Code, w.Body.String())
		}
	})

	t.Run("PUT /api/services/{id} par un tiers → 404", func(t *testing.T) {
		body := `{"titre":"HACKED","categorie":"Informatique","duree_minutes":90,"credits":8}`
		w := doReq("PUT", "/api/services/"+strconv.Itoa(serviceID), body, providerID+9999)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404 — %s", w.Code, w.Body.String())
		}
	})

	t.Run("DELETE /api/services/{id} → 204", func(t *testing.T) {
		providerID2, serviceID2 := setupServiceFixture(t)
		w := doReq("DELETE", "/api/services/"+strconv.Itoa(serviceID2), "", providerID2)
		if w.Code != http.StatusNoContent {
			t.Fatalf("got %d, want 204 — %s", w.Code, w.Body.String())
		}
	})
}

func TestGetStatsHTTP(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	mux := http.NewServeMux()
	RegisterUserRoutes(mux, db)
	RegisterServiceRoutes(mux, db)

	u, _ := insertUser(ctx, db, User{Pseudo: "stats-user"})

	req := httptest.NewRequest("GET", "/api/users/"+strconv.Itoa(u.ID)+"/stats", nil)
	req.Header.Set("X-User-ID", strconv.Itoa(u.ID))
	w := httptest.NewRecorder()
	AuthMiddleware(mux).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d, want 200 — body: %s", w.Code, w.Body.String())
	}
	var stats UserStats
	json.NewDecoder(w.Body).Decode(&stats)
	if stats.CreditBalance != 10 {
		t.Errorf("credit_balance = %d, want 10 (bienvenue)", stats.CreditBalance)
	}
}

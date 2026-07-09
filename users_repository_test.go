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

func TestInsertAndSelectUser(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	u, err := insertUser(ctx, db, User{Pseudo: "testuser", Bio: "bio", Ville: "Paris"})
	if err != nil {
		t.Fatalf("insertUser: %v", err)
	}
	if u.ID == 0 {
		t.Error("ID attendu non nul")
	}
	if u.CreditBalance != 10 {
		t.Errorf("credit_balance = %d, want 10", u.CreditBalance)
	}

	got, err := selectUser(ctx, db, u.ID)
	if err != nil {
		t.Fatalf("selectUser: %v", err)
	}
	if got.Pseudo != "testuser" {
		t.Errorf("pseudo = %q, want %q", got.Pseudo, "testuser")
	}
}

func TestSelectUserNotFound(t *testing.T) {
	db := testDB(t)
	_, err := selectUser(context.Background(), db, 999999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("attendu ErrNotFound, got %v", err)
	}
}

func TestUpdateUserRow(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	u, _ := insertUser(ctx, db, User{Pseudo: "avant"})
	u.Pseudo = "après"
	if err := updateUserRow(ctx, db, *u); err != nil {
		t.Fatalf("updateUserRow: %v", err)
	}
	got, _ := selectUser(ctx, db, u.ID)
	if got.Pseudo != "après" {
		t.Errorf("pseudo = %q, want %q", got.Pseudo, "après")
	}
	if got.CreditBalance != 10 {
		t.Error("updateUserRow ne doit pas modifier credit_balance")
	}
}

func TestReplaceSkillsRows(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	u, _ := insertUser(ctx, db, User{Pseudo: "skilluser"})

	skills3 := []Skill{{Nom: "Go", Niveau: "expert"}, {Nom: "SQL", Niveau: "intermédiaire"}, {Nom: "Docker", Niveau: "débutant"}}
	if err := replaceSkillsRows(ctx, db, u.ID, skills3); err != nil {
		t.Fatalf("replaceSkillsRows (3): %v", err)
	}
	got, _ := selectSkills(ctx, db, u.ID)
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}

	// Écrasement : 1 seule skill
	skills1 := []Skill{{Nom: "Go", Niveau: "expert"}}
	if err := replaceSkillsRows(ctx, db, u.ID, skills1); err != nil {
		t.Fatalf("replaceSkillsRows (1): %v", err)
	}
	got, _ = selectSkills(ctx, db, u.ID)
	if len(got) != 1 {
		t.Errorf("après écrasement len = %d, want 1", len(got))
	}
}

// --- Tests httptest bout-en-bout ---

func TestCreateUserHTTP(t *testing.T) {
	db := testDB(t)
	mux := http.NewServeMux()
	RegisterUserRoutes(mux, db)
	mux.HandleFunc("POST /api/users", createUserHandler(db))

	t.Run("201 credit_balance=10 (test #1)", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/users", strings.NewReader(`{"pseudo":"httpuser","ville":"Lyon"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201 — body: %s", w.Code, w.Body.String())
		}
		var u User
		json.NewDecoder(w.Body).Decode(&u)
		if u.CreditBalance != 10 {
			t.Errorf("credit_balance = %d, want 10", u.CreditBalance)
		}
	})

	t.Run("400 pseudo vide (test #2)", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/users", strings.NewReader(`{"pseudo":""}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", w.Code)
		}
	})
}

func TestPutSkillsHTTP(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	mux := http.NewServeMux()
	RegisterUserRoutes(mux, db)

	u, _ := insertUser(ctx, db, User{Pseudo: "skillhttp"})

	doReq := func(userID int, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("PUT", "/api/users/"+strconv.Itoa(u.ID)+"/skills", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", strconv.Itoa(userID))
		w := httptest.NewRecorder()
		AuthMiddleware(mux).ServeHTTP(w, req)
		return w
	}

	t.Run("400 niveau invalide", func(t *testing.T) {
		w := doReq(u.ID, `[{"nom":"Go","niveau":"dieu"}]`)
		if w.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400", w.Code)
		}
	})

	t.Run("401 modification par autrui", func(t *testing.T) {
		w := doReq(u.ID+9999, `[{"nom":"Go","niveau":"expert"}]`)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", w.Code)
		}
	})
}

func TestGetAndUpdateUserHTTP(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	mux := http.NewServeMux()
	RegisterUserRoutes(mux, db)

	u, _ := insertUser(ctx, db, User{Pseudo: "getuser-http", Ville: "Paris"})

	doReq := func(method, path, body string, userID int) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", strconv.Itoa(userID))
		w := httptest.NewRecorder()
		AuthMiddleware(mux).ServeHTTP(w, req)
		return w
	}

	t.Run("GET /api/users/{id} → 200", func(t *testing.T) {
		w := doReq("GET", "/api/users/"+strconv.Itoa(u.ID), "", u.ID)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 — %s", w.Code, w.Body.String())
		}
		var got User
		json.NewDecoder(w.Body).Decode(&got)
		if got.Pseudo != "getuser-http" {
			t.Errorf("pseudo = %q, want getuser-http", got.Pseudo)
		}
	})

	t.Run("GET /api/users/{id} inexistant → 404", func(t *testing.T) {
		w := doReq("GET", "/api/users/999999", "", u.ID)
		if w.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", w.Code)
		}
	})

	t.Run("PUT /api/users/{id} → 200", func(t *testing.T) {
		w := doReq("PUT", "/api/users/"+strconv.Itoa(u.ID), `{"pseudo":"updated-http","ville":"Lyon"}`, u.ID)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 — %s", w.Code, w.Body.String())
		}
	})

	t.Run("PUT /api/users/{id} par autrui → 401", func(t *testing.T) {
		w := doReq("PUT", "/api/users/"+strconv.Itoa(u.ID), `{"pseudo":"pirate"}`, u.ID+9999)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", w.Code)
		}
	})

	t.Run("GET /api/users/{id}/skills → 200", func(t *testing.T) {
		w := doReq("GET", "/api/users/"+strconv.Itoa(u.ID)+"/skills", "", u.ID)
		if w.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 — %s", w.Code, w.Body.String())
		}
	})

	_ = ctx
}

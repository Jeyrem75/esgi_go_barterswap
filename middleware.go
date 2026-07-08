package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

type contextKey string

const userIDKey contextKey = "userID"

// LoggingMiddleware journalise la méthode, le chemin et la durée de chaque requête.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// RecoveryMiddleware intercepte les panics et retourne une réponse 500 au lieu de crasher.
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic récupérée: %v", rec)
				writeError(w, fmt.Errorf("erreur interne"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware ajoute les headers CORS et gère les requêtes OPTIONS preflight.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-User-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthMiddleware vérifie la présence et la validité du header X-User-ID et injecte l'ID en contexte.
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.Header.Get("X-User-ID")
		if idStr == "" {
			writeError(w, fmt.Errorf("header X-User-ID manquant: %w", ErrUnauthorized))
			return
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			writeError(w, fmt.Errorf("X-User-ID invalide: %w", ErrUnauthorized))
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserIDFromContext extrait l'ID utilisateur déposé par AuthMiddleware.
func GetUserIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userIDKey).(int)
	return id, ok
}

// TimeoutMiddleware annule le contexte de chaque requête après la durée donnée.
func TimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Querier est satisfaite implicitement par *sql.DB ET *sql.Tx — les deux exposent déjà
// ces méthodes. On peut donc appeler ces fonctions avec une connexion simple ou depuis
// l'intérieur d'une transaction, sans dupliquer le code pour les deux cas.
type Querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// fetchService lit une annonce de service complète depuis une connexion ou une transaction.
func fetchService(ctx context.Context, q Querier, id int) (*Service, error) {
	var s Service
	err := q.QueryRowContext(ctx, `
        SELECT id, provider_id, titre, description, categorie, duree_minutes, credits, ville, actif, created_at
        FROM services WHERE id = $1`, id).Scan(
		&s.ID, &s.ProviderID, &s.Titre, &s.Description, &s.Categorie,
		&s.DureeMinutes, &s.Credits, &s.Ville, &s.Actif, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("service %d: %w", id, ErrNotFound)
	}
	return &s, err
}

// fetchUserBalance lit le solde de crédits d'un utilisateur.
func fetchUserBalance(ctx context.Context, q Querier, userID int) (int, error) {
	var balance int
	err := q.QueryRowContext(ctx, `SELECT credit_balance FROM users WHERE id = $1`, userID).Scan(&balance)
	return balance, err
}

// userHasSkill vérifie qu'un utilisateur a déclaré une compétence dans la catégorie donnée.
func userHasSkill(ctx context.Context, q Querier, userID int, skillNom string) (bool, error) {
	var exists bool
	err := q.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM skills WHERE user_id = $1 AND nom = $2)`,
		userID, skillNom).Scan(&exists)
	return exists, err
}

// hasActiveExchangeForService vérifie qu'un service n'a pas déjà un échange pending ou accepted.
func hasActiveExchangeForService(ctx context.Context, q Querier, serviceID int) (bool, error) {
	var exists bool
	err := q.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM exchanges WHERE service_id = $1 AND status IN ('pending','accepted'))`,
		serviceID).Scan(&exists)
	return exists, err
}

// fetchExchangeStatus lit le statut courant d'un échange.
func fetchExchangeStatus(ctx context.Context, q Querier, exchangeID int) (string, error) {
	var status string
	err := q.QueryRowContext(ctx, `SELECT status FROM exchanges WHERE id = $1`, exchangeID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("échange %d: %w", exchangeID, ErrNotFound)
	}
	return status, err
}

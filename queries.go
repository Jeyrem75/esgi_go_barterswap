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
// COALESCE sur description/ville : ces colonnes sont nullables en base (migrations/001_init.sql),
// et Service.Description/Ville sont des string Go brutes — sans ça, un service inséré sans ces
// champs fait planter le Scan pour tout appelant (getServiceHandler, createExchange, ...).
func fetchService(ctx context.Context, q Querier, id int) (*Service, error) {
	var s Service
	err := q.QueryRowContext(ctx, `
        SELECT id, provider_id, titre, COALESCE(description, ''), categorie, duree_minutes, credits, COALESCE(ville, ''), actif, created_at
        FROM services WHERE id = $1`, id).Scan(
		&s.ID, &s.ProviderID, &s.Titre, &s.Description, &s.Categorie,
		&s.DureeMinutes, &s.Credits, &s.Ville, &s.Actif, &s.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("service %d: %w", id, ErrNotFound)
	}
	return &s, err
}

// fetchServiceForUpdate verrouille le service (FOR UPDATE) pour sérialiser deux créations d'échange concurrentes.
func fetchServiceForUpdate(ctx context.Context, q Querier, id int) (*Service, error) {
	var s Service
	err := q.QueryRowContext(ctx, `
        SELECT id, provider_id, titre, COALESCE(description, ''), categorie, duree_minutes, credits, COALESCE(ville, ''), actif, created_at
        FROM services WHERE id = $1 FOR UPDATE`, id).Scan(
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

// fetchExchangeParties lit les deux participants et le statut d'un échange. Lecture
// cross-domaine utilisée pour dériver la cible d'un avis ; l'écriture sur exchanges
// reste réservée à exchanges.go.
func fetchExchangeParties(ctx context.Context, q Querier, exchangeID int) (requesterID, ownerID int, status string, err error) {
	err = q.QueryRowContext(ctx,
		`SELECT requester_id, owner_id, status FROM exchanges WHERE id = $1`, exchangeID).
		Scan(&requesterID, &ownerID, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, "", fmt.Errorf("échange %d: %w", exchangeID, ErrNotFound)
	}
	return requesterID, ownerID, status, err
}

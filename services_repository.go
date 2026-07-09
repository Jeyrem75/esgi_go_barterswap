package main

import (
	"context"
	"database/sql"
	"fmt"
)

func insertServiceRow(ctx context.Context, db *sql.DB, s Service) (*Service, error) {
	row := db.QueryRowContext(ctx,
		`INSERT INTO services (provider_id, titre, description, categorie, duree_minutes, credits, ville, actif)
         VALUES ($1,$2,$3,$4,$5,$6,$7,true)
         RETURNING id, created_at`,
		s.ProviderID, s.Titre, s.Description, s.Categorie, s.DureeMinutes, s.Credits, s.Ville)
	if err := row.Scan(&s.ID, &s.CreatedAt); err != nil {
		return nil, err
	}
	s.Actif = true
	return &s, nil
}

// updateServiceRow renvoie le nombre de lignes affectées — c'est à la couche métier
// de décider que 0 ligne affectée = ErrNotFound, pas au repository (même pattern que deleteServiceRow).
func updateServiceRow(ctx context.Context, db *sql.DB, s Service) (int64, error) {
	res, err := db.ExecContext(ctx, `
        UPDATE services SET titre=$1, description=$2, categorie=$3, duree_minutes=$4,
               credits=$5, ville=$6 WHERE id=$7 AND provider_id=$8`,
		s.Titre, s.Description, s.Categorie, s.DureeMinutes, s.Credits, s.Ville, s.ID, s.ProviderID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// deleteServiceRow renvoie le nombre de lignes affectées — c'est à la couche métier
// de décider que 0 ligne affectée = ErrNotFound, pas au repository.
func deleteServiceRow(ctx context.Context, db *sql.DB, serviceID, providerID int) (int64, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM services WHERE id = $1 AND provider_id = $2`, serviceID, providerID)
	if err != nil {
		if isForeignKeyViolation(err) {
			return 0, fmt.Errorf("service %d référencé par un échange existant: %w", serviceID, ErrConflict)
		}
		return 0, err
	}
	return res.RowsAffected()
}

func listServiceRows(ctx context.Context, db *sql.DB, categorie, ville, search string) ([]Service, error) {
	query := `SELECT id, provider_id, titre, description, categorie, duree_minutes, credits, ville, actif, created_at
              FROM services WHERE actif = true`
	var args []any
	if categorie != "" {
		args = append(args, categorie)
		query += fmt.Sprintf(" AND categorie = $%d", len(args))
	}
	if ville != "" {
		args = append(args, ville)
		query += fmt.Sprintf(" AND ville = $%d", len(args))
	}
	if search != "" {
		args = append(args, "%"+search+"%")
		query += fmt.Sprintf(" AND (titre ILIKE $%d OR description ILIKE $%d)", len(args), len(args))
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var services []Service
	for rows.Next() {
		var s Service
		if err := rows.Scan(&s.ID, &s.ProviderID, &s.Titre, &s.Description, &s.Categorie,
			&s.DureeMinutes, &s.Credits, &s.Ville, &s.Actif, &s.CreatedAt); err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	return services, rows.Err()
}

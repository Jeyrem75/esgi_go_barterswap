# BarterSwap — API d'échange de compétences

API REST de banque de temps : les utilisateurs échangent leurs compétences via un système de crédits, sans argent.

## Installation

```bash
git clone <url>
cd esgi_go_barterswap
go mod tidy

# Démarrer PostgreSQL via Docker
docker-compose up -d

# Appliquer les migrations
psql "postgres://barterswap:barterswap@localhost:5433/barterswap" -f migrations/001_init.sql

# Lancer l'API
DATABASE_URL="postgres://barterswap:barterswap@localhost:5433/barterswap?sslmode=disable" go run .
```

L'API écoute sur `:8080`. Toutes les routes (sauf `/health` et `POST /api/users`) requièrent le header `X-User-ID: <id>`.

## Architecture

Un seul package `main` (contrainte du sujet), séparé par fichier plutôt que par dossier :

- `main.go` — connexion DB, routes, middlewares
- `models.go` — structs (`User`, `Service`, `Exchange`, `Review`...)
- `users.go` / `services.go` / `exchanges.go` / `reviews.go` — handlers HTTP + logique métier
- `*_repository.go` — requêtes SQL uniquement, pas de règles métier
- `queries.go` — lectures partagées entre domaines (interface `Querier`, marche avec `*sql.DB` ou `*sql.Tx`)
- `credits.go` — logique des transactions de crédits
- `errors.go` — sentinelles d'erreurs + mapping erreur → code HTTP
- `middleware.go` — logging, recovery, CORS, timeout, auth

Les handlers appellent des fonctions métier (ex. `createExchange`), qui appellent les fonctions du repository. Pas de SQL dans les handlers, pas de logique métier dans les repositories.

## Endpoints

| Méthode  | Path                              | Description                          |
|----------|-----------------------------------|--------------------------------------|
| `GET`    | `/health`                         | Santé de l'API                       |
| `POST`   | `/api/users`                      | Créer un compte (10 crédits offerts) |
| `GET`    | `/api/users/{id}`                 | Profil public                        |
| `PUT`    | `/api/users/{id}`                 | Modifier son profil                  |
| `GET`    | `/api/users/{id}/skills`          | Compétences d'un utilisateur         |
| `PUT`    | `/api/users/{id}/skills`          | Définir ses compétences              |
| `GET`    | `/api/users/{id}/reviews`         | Avis reçus par un utilisateur        |
| `GET`    | `/api/users/{id}/stats`           | Statistiques d'un utilisateur        |
| `GET`    | `/api/services`                   | Lister les services (filtres optionnels) |
| `POST`   | `/api/services`                   | Publier une annonce                  |
| `GET`    | `/api/services/{id}`              | Détail d'un service                  |
| `PUT`    | `/api/services/{id}`              | Modifier son annonce                 |
| `DELETE` | `/api/services/{id}`              | Supprimer son annonce                |
| `GET`    | `/api/services/{id}/reviews`      | Avis sur un service                  |
| `POST`   | `/api/exchanges`                  | Créer une demande d'échange          |
| `GET`    | `/api/exchanges`                  | Mes échanges (envoyés + reçus)       |
| `GET`    | `/api/exchanges/{id}`             | Détail d'un échange                  |
| `PUT`    | `/api/exchanges/{id}/accept`      | Accepter une demande                 |
| `PUT`    | `/api/exchanges/{id}/reject`      | Refuser une demande                  |
| `PUT`    | `/api/exchanges/{id}/complete`    | Confirmer la prestation              |
| `PUT`    | `/api/exchanges/{id}/cancel`      | Annuler un échange                   |
| `POST`   | `/api/exchanges/{id}/review`      | Laisser un avis (échange terminé)    |

Filtres disponibles sur `GET /api/services` : `?categorie=`, `?ville=`, `?search=`
Filtre disponible sur `GET /api/exchanges` : `?status=`

## Exemples d'utilisation

### 1. Créer un utilisateur

```bash
curl -s -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"pseudo":"alice","ville":"Paris","bio":"Dev Go"}' | jq
```

### 2. Définir ses compétences et publier un service

```bash
# Ajouter une compétence
curl -s -X PUT http://localhost:8080/api/users/1/skills \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -d '[{"nom":"Informatique","niveau":"expert"}]'

# Publier le service (la catégorie doit correspondre à une compétence déclarée)
curl -s -X POST http://localhost:8080/api/services \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 1" \
  -d '{"titre":"Cours Go","categorie":"Informatique","duree_minutes":60,"credits":5}' | jq
```

### 3. Créer et accepter un échange

```bash
# L'utilisateur 2 demande un échange sur le service 1
curl -s -X POST http://localhost:8080/api/exchanges \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 2" \
  -d '{"service_id":1}' | jq

# L'utilisateur 1 (propriétaire du service) accepte : crédits bloqués côté demandeur
curl -s -X PUT http://localhost:8080/api/exchanges/1/accept \
  -H "X-User-ID: 1"

# L'utilisateur 2 (demandeur) confirme la prestation rendue : crédits transférés à l'offreur
curl -s -X PUT http://localhost:8080/api/exchanges/1/complete \
  -H "X-User-ID: 2"
```

### 4. Laisser un avis

```bash
# La cible de l'avis est déduite côté serveur (l'autre participant de l'échange) —
# pas besoin (et pas possible) de la préciser dans le body.
curl -s -X POST http://localhost:8080/api/exchanges/1/review \
  -H "Content-Type: application/json" \
  -H "X-User-ID: 2" \
  -d '{"note":5,"commentaire":"Excellent cours, très pédagogue !"}' | jq
```

## Tests

```bash
DATABASE_URL="postgres://barterswap:barterswap@localhost:5433/barterswap?sslmode=disable" \
  go test -v -cover ./...
```

Couverture actuelle : **~63%** (seuil requis : 60%), suite verte y compris avec `-race`.

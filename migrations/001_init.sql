CREATE TABLE users (
    id             SERIAL PRIMARY KEY,
    pseudo         TEXT NOT NULL,
    bio            TEXT,
    ville          TEXT,
    credit_balance INT NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE skills (
    id      SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nom     TEXT NOT NULL,
    niveau  TEXT NOT NULL CHECK (niveau IN ('débutant', 'intermédiaire', 'expert'))
);

CREATE TABLE services (
    id            SERIAL PRIMARY KEY,
    provider_id   INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    titre         TEXT NOT NULL,
    description   TEXT,
    categorie     TEXT NOT NULL,
    duree_minutes INT NOT NULL,
    credits       INT NOT NULL,
    ville         TEXT,
    actif         BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE exchanges (
    id           SERIAL PRIMARY KEY,
    service_id   INT NOT NULL REFERENCES services(id),
    requester_id INT NOT NULL REFERENCES users(id),
    owner_id     INT NOT NULL REFERENCES users(id),
    status       TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending','accepted','rejected','cancelled','completed')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE credit_transactions (
    id          SERIAL PRIMARY KEY,
    user_id     INT NOT NULL REFERENCES users(id),
    exchange_id INT REFERENCES exchanges(id),
    montant     INT NOT NULL,
    type        TEXT NOT NULL CHECK (type IN ('earn','spend','refund')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE reviews (
    id          SERIAL PRIMARY KEY,
    exchange_id INT NOT NULL REFERENCES exchanges(id),
    author_id   INT NOT NULL REFERENCES users(id),
    target_id   INT NOT NULL REFERENCES users(id),
    note        INT NOT NULL CHECK (note BETWEEN 1 AND 5),
    commentaire TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (exchange_id, author_id)
);

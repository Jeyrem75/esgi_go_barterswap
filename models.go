package main

type User struct {
	ID            int     `json:"id"`
	Pseudo        string  `json:"pseudo"`
	Bio           string  `json:"bio,omitempty"`
	Ville         string  `json:"ville,omitempty"`
	Skills        []Skill `json:"skills,omitempty"`
	CreditBalance int     `json:"credit_balance"`
	CreatedAt     string  `json:"created_at"`
}

type Skill struct {
	Nom    string `json:"nom"`
	Niveau string `json:"niveau"` // "débutant", "intermédiaire", "expert"
}

type Service struct {
	ID           int    `json:"id"`
	ProviderID   int    `json:"provider_id"`
	Titre        string `json:"titre"`
	Description  string `json:"description,omitempty"`
	Categorie    string `json:"categorie"`
	DureeMinutes int    `json:"duree_minutes"`
	Credits      int    `json:"credits"`
	Ville        string `json:"ville,omitempty"`
	Actif        bool   `json:"actif"`
	CreatedAt    string `json:"created_at"`
}

type Exchange struct {
	ID          int    `json:"id"`
	ServiceID   int    `json:"service_id"`
	RequesterID int    `json:"requester_id"`
	OwnerID     int    `json:"owner_id"`
	Status      string `json:"status"` // pending, accepted, rejected, cancelled, completed
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type CreditTransaction struct {
	ID         int    `json:"id"`
	UserID     int    `json:"user_id"`
	ExchangeID int    `json:"exchange_id"`
	Montant    int    `json:"montant"` // positif = crédit, négatif = débit
	Type       string `json:"type"`    // "earn", "spend", "refund"
	CreatedAt  string `json:"created_at"`
}

type Review struct {
	ID          int    `json:"id"`
	ExchangeID  int    `json:"exchange_id"`
	AuthorID    int    `json:"author_id"`
	TargetID    int    `json:"target_id"`
	Note        int    `json:"note"` // 1-5
	Commentaire string `json:"commentaire,omitempty"`
	CreatedAt   string `json:"created_at"`
}

type UserStats struct {
	UserID            int     `json:"user_id"`
	ServicesActifs    int     `json:"services_actifs"`
	EchangesCompletes int     `json:"echanges_completes"`
	CreditBalance     int     `json:"credit_balance"`
	NoteMoyenne       float64 `json:"note_moyenne"`
	NbAvis            int     `json:"nb_avis"`
	TotalGagne        int     `json:"total_gagne"`
	TotalDepense      int     `json:"total_depense"`
}

// CategoriesValides est la liste fermée des catégories acceptées pour un service.
var CategoriesValides = map[string]bool{
	"Informatique": true, "Jardinage": true, "Bricolage": true, "Cuisine": true,
	"Musique": true, "Langues": true, "Sport": true, "Tutorat": true,
	"Déménagement": true, "Photographie": true, "Animalier": true,
	"Couture": true, "Autre": true,
}

// NiveauxValides est la liste fermée des niveaux acceptés pour une compétence.
var NiveauxValides = map[string]bool{"débutant": true, "intermédiaire": true, "expert": true}

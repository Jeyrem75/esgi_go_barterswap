package main

import (
	"errors"
	"testing"
)

func TestValidateService(t *testing.T) {
	tests := []struct {
		name     string
		service  Service
		hasSkill bool
		wantErr  bool
		errIs    error
	}{
		{"valide", Service{Titre: "Cours Go", Categorie: "Informatique"}, true, false, nil},
		{"catégorie invalide", Service{Titre: "Cours Go", Categorie: "Magie"}, true, true, ErrValidation},
		{"titre vide", Service{Titre: "", Categorie: "Informatique"}, true, true, ErrValidation},
		{"titre espaces", Service{Titre: "   ", Categorie: "Informatique"}, true, true, ErrValidation},
		{"compétence manquante", Service{Titre: "Cours Go", Categorie: "Informatique"}, false, true, ErrValidation},
		{"toutes catégories valides - Jardinage", Service{Titre: "Taille haie", Categorie: "Jardinage"}, true, false, nil},
		{"toutes catégories valides - Sport", Service{Titre: "Tennis", Categorie: "Sport"}, true, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateService(tt.service, tt.hasSkill)
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.errIs != nil && !errors.Is(err, tt.errIs) {
				t.Errorf("attendu %v, got %v", tt.errIs, err)
			}
		})
	}
}

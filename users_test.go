package main

import (
	"errors"
	"testing"
)

func TestValidateNewUser(t *testing.T) {
	tests := []struct {
		name    string
		user    User
		wantErr bool
	}{
		{"pseudo valide", User{Pseudo: "alice"}, false},
		{"pseudo avec espaces", User{Pseudo: "alice bob"}, false},
		{"pseudo vide", User{Pseudo: ""}, true},
		{"pseudo espaces seuls", User{Pseudo: "   "}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNewUser(tt.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrValidation) {
				t.Errorf("attendu ErrValidation, got %v", err)
			}
		})
	}
}

func TestValidateSkills(t *testing.T) {
	tests := []struct {
		name    string
		skills  []Skill
		wantErr bool
	}{
		{"liste vide", []Skill{}, false},
		{"débutant valide", []Skill{{Nom: "Go", Niveau: "débutant"}}, false},
		{"intermédiaire valide", []Skill{{Nom: "Go", Niveau: "intermédiaire"}}, false},
		{"expert valide", []Skill{{Nom: "Go", Niveau: "expert"}}, false},
		{"niveau invalide", []Skill{{Nom: "Go", Niveau: "dieu"}}, true},
		{"mix valide et invalide", []Skill{{Nom: "Go", Niveau: "expert"}, {Nom: "JS", Niveau: "wizard"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSkills(tt.skills)
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrValidation) {
				t.Errorf("attendu ErrValidation, got %v", err)
			}
		})
	}
}

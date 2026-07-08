package main

import (
	"errors"
	"testing"
)

func TestValidateReview(t *testing.T) {
	tests := []struct {
		name    string
		status  string
		note    int
		wantErr bool
		errIs   error
	}{
		{"completed note 3", "completed", 3, false, nil},
		{"completed note 1", "completed", 1, false, nil},
		{"completed note 5", "completed", 5, false, nil},
		{"pending", "pending", 3, true, ErrValidation},
		{"accepted", "accepted", 3, true, ErrValidation},
		{"rejected", "rejected", 3, true, ErrValidation},
		{"cancelled", "cancelled", 3, true, ErrValidation},
		{"note 0", "completed", 0, true, ErrValidation},
		{"note 6", "completed", 6, true, ErrValidation},
		{"note négative", "completed", -1, true, ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReview(tt.status, tt.note)
			if (err != nil) != tt.wantErr {
				t.Errorf("got err=%v, wantErr=%v", err, tt.wantErr)
			}
			if tt.errIs != nil && !errors.Is(err, tt.errIs) {
				t.Errorf("attendu %v, got %v", tt.errIs, err)
			}
		})
	}
}

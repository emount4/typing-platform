package session

import (
	"github.com/emount4/typing-realtime/internal/domain"
)

type Validator struct {
}

func NewValidaator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidateKS(sess *Session, char string, timestamp int64) domain.ValidationResult {
	if char == "Backspace" {
		if sess.Position >= len(sess.TextRunes) {
			return domain.ValidationResult{
				IsValid: false,
				Reason:  "text already completed",
			}
		}

		return domain.ValidationResult{IsValid: true}
	}

	if sess.Position >= len(sess.TextRunes) {
		return domain.ValidationResult{
			IsValid: false,
			Reason:  "text already completed",
		}
	}

	expectedChar := string(sess.TextRunes[sess.Position])
	isError := char != expectedChar

	return domain.ValidationResult{
		IsValid: true,
		IsError: isError,
	}
}

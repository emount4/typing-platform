package session

import (
	"fmt"

	"github.com/emount4/typing-realtime/internal/domain"
)

const (
	MinIntervalMs = 15
)

type Validator struct {
}

func NewValidaator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidateKS(sess *Session, char string, timestamp int64) domain.ValidationResult {
	if len(sess.Keystrokes) > 0 {

		lastTimestamp := sess.Keystrokes[len(sess.Keystrokes)-1].Timestamp

		if timestamp <= lastTimestamp {
			return domain.ValidationResult{
				IsValid: false,
				Reason:  "timestamp not monotonic",
			}
		}

		interval := timestamp - lastTimestamp

		if interval < MinIntervalMs {
			return domain.ValidationResult{
				IsValid: false,
				Reason:  fmt.Sprintf("intervals too low: %dms < %dms", interval, MinIntervalMs),
			}
		}
	}

	if sess.Position >= len(sess.Text) {
		return domain.ValidationResult{
			IsValid: false,
			Reason:  "text already completed",
		}
	}

	expectedChar := string(sess.Text[sess.Position])
	isError := char != expectedChar

	return domain.ValidationResult{
		IsValid: true,
		IsError: isError,
	}
}

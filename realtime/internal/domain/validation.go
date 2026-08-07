package domain

type ValidationResult struct {
	IsValid bool
	IsError bool
	Reason  string
}

package domain

type Keystroke struct {
	Char      string `json:"char"`
	Timestamp int64  `json:"timestamp"` // миллисекунды
	IsError   bool   // Ошибка или нет
	Interval  int64  // Интервал от предыдущего нажатия (мс)
}

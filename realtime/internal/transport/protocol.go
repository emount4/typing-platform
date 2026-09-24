package transport

type AuthMessage struct {
	Type   string  `json:"type"`
	V      int     `json:"v"`
	Token  *string `json:"token"`
	AnonID *string `json:"anon_id"`
}

type BaseMessage struct {
	Type string `json:"type"`
}

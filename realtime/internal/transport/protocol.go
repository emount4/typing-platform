package transport

type AuthMessage struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

type BaseMessage struct {
	Type string `json:"type"`
}

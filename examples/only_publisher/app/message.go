package app

import "encoding/json"

type Message struct {
	UserID  string `json:"user_id"`
	Message string `json:"message"`
}

func BuildPayload() ([]byte, error) {
	message := Message{
		UserID:  "user_123",
		Message: "hello from only_publisher",
	}

	return json.Marshal(message)
}

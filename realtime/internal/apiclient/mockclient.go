package apiclient

import (
	"context"
	"log"
)

type MockAPIClient struct{}

func NewMockAPIClient() *MockAPIClient {
	return &MockAPIClient{}
}

func (m *MockAPIClient) GetText(ctx context.Context, textID string) (*TextResponse, error) {
	// Возвращаем захардкоженный тестовый текст
	return &TextResponse{
		ID:      textID,
		Content: "Это тестовый текст для проверки работы одиночного режима. Он должен содержать достаточно символов.",
		Lang:    "ru",
	}, nil
}

func (m *MockAPIClient) SubmitRun(ctx context.Context, run *RunResult) error {
	// Просто логируем, что результат отправлен
	log.Printf("📤 Mock: отправка результата в Python API: WPM=%.2f, Accuracy=%.2f", run.WPM, run.Accuracy)
	return nil
}

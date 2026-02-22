package query

import (
	"context"
	"errors"
	"testing"

	"github.com/booksage/booksage-api/internal/domain/repository"
)

type mockClient struct {
	name string
	resp string
	err  error
}

func (m *mockClient) Generate(ctx context.Context, prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.resp, nil
}

func (m *mockClient) Name() string { return m.name }

type mockRouter struct {
	local  repository.LLMClient
	gemini repository.LLMClient
}

func (m *mockRouter) RouteLLMTask(task repository.TaskType) repository.LLMClient {
	if task == repository.TaskType("agentic_reasoning") {
		return m.gemini
	}
	return m.local
}

func TestGenerateAnswer_Success(t *testing.T) {
	local := &mockClient{name: "local", resp: "keyword1, keyword2"}
	gemini := &mockClient{name: "gemini", resp: "Final reasoned answer"}
	router := &mockRouter{local: local, gemini: gemini}

	gen := NewGenerator(router, nil)

	eventStream := make(chan GeneratorEvent, 10)
	go gen.GenerateAnswer(context.Background(), "test query", eventStream)

	var events []GeneratorEvent
	for ev := range eventStream {
		events = append(events, ev)
	}

	if len(events) < 4 {
		t.Fatalf("Expected at least 4 events, got %d", len(events))
	}

	if events[len(events)-1].Type != "answer" {
		t.Errorf("Expected last event to be answer, got %s", events[len(events)-1].Type)
	}
}

func TestGenerateAnswer_LocalFails(t *testing.T) {
	local := &mockClient{name: "local", err: errors.New("local error")}
	gemini := &mockClient{name: "gemini", resp: "Final reasoned answer"}
	router := &mockRouter{local: local, gemini: gemini}

	gen := NewGenerator(router, nil)

	eventStream := make(chan GeneratorEvent, 20)
	go gen.GenerateAnswer(context.Background(), "test query", eventStream)

	var lastEvent GeneratorEvent
	for ev := range eventStream {
		lastEvent = ev
	}

	if lastEvent.Type != "answer" {
		t.Errorf("Expected final event to be answer (keyword failure is non-fatal), got %s: %s", lastEvent.Type, lastEvent.Content)
	}
}

func TestGenerateAnswer_GeminiFails(t *testing.T) {
	local := &mockClient{name: "local", resp: "keyword"}
	gemini := &mockClient{name: "gemini", err: errors.New("gemini error")}
	router := &mockRouter{local: local, gemini: gemini}

	gen := NewGenerator(router, nil)

	eventStream := make(chan GeneratorEvent, 10)
	go gen.GenerateAnswer(context.Background(), "test query", eventStream)

	var lastEvent GeneratorEvent
	for ev := range eventStream {
		lastEvent = ev
	}

	if lastEvent.Type != "error" {
		t.Errorf("Expected final event to be error, got %s", lastEvent.Type)
	}
}

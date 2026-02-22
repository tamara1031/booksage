package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/booksage/booksage-api/internal/llm"
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

func TestGenerateAnswer_Success(t *testing.T) {
	local := &mockClient{name: "local", resp: "keyword1, keyword2"}
	gemini := &mockClient{name: "gemini", resp: "Final reasoned answer"}
	router := llm.NewRouter(local, gemini)

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
	if events[len(events)-1].Content != "Final reasoned answer" {
		t.Errorf("Expected answer content, got %s", events[len(events)-1].Content)
	}
}

func TestGenerateAnswer_LocalFails(t *testing.T) {
	local := &mockClient{name: "local", err: errors.New("local error")}
	gemini := &mockClient{name: "gemini", resp: "Final reasoned answer"}
	router := llm.NewRouter(local, gemini)

	gen := NewGenerator(router, nil)

	eventStream := make(chan GeneratorEvent, 20)
	go gen.GenerateAnswer(context.Background(), "test query", eventStream)

	var lastEvent GeneratorEvent
	for ev := range eventStream {
		lastEvent = ev
	}

	// Keyword extraction failure is non-fatal (falls back to raw query),
	// so the final event should be an answer from gemini.
	if lastEvent.Type != "answer" {
		t.Errorf("Expected final event to be answer (keyword failure is non-fatal), got %s: %s", lastEvent.Type, lastEvent.Content)
	}
}

func TestGenerateAnswer_GeminiFails(t *testing.T) {
	local := &mockClient{name: "local", resp: "keyword"}
	gemini := &mockClient{name: "gemini", err: errors.New("gemini error")}
	router := llm.NewRouter(local, gemini)

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

func TestAgentOrchestrator(t *testing.T) {
	local := &mockClient{name: "local"}
	gemini := &mockClient{name: "gemini"}
	router := llm.NewRouter(local, gemini)

	orch := NewAgentOrchestrator(router)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	resp, err := orch.Run(ctx, "test query")
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if resp != "Mock Answer" {
		t.Errorf("Expected Mock Answer, got %s", resp)
	}
}

func TestRouteLLMTaskAlias(t *testing.T) {
	local := &mockClient{name: "local_alias"}
	gemini := &mockClient{name: "gemini_alias"}
	router := llm.NewRouter(local, gemini)

	client := RouteLLMTask(router, llm.TaskSimpleKeywordExtraction)
	if client.Name() != "local_alias" {
		t.Errorf("Expected local_alias, got %s", client.Name())
	}
}

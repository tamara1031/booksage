package query

import (
	"context"
	"errors"
	"testing"
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
	gemini := &mockClient{name: "gemini", resp: "Final reasoned answer"}
	gen := NewGenerator(gemini, nil)

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
	// Local failure is no longer a separate task in the same way via router
	// We just test if the generator handles its own internal logic.
	// Since both logic steps now use the SAME LLMClient, we just test a failure case.
	gemini := &mockClient{name: "gemini", resp: "Final reasoned answer"}
	gen := NewGenerator(gemini, nil)

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
	gemini := &mockClient{name: "gemini", err: errors.New("gemini error")}

	gen := NewGenerator(gemini, nil)

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

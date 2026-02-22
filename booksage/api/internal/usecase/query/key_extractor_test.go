package query

import (
	"context"
	"testing"
)

func TestDualKeyExtractor_ExtractKeys(t *testing.T) {
	tests := []struct {
		name           string
		resp           string
		expectedEnts   int
		expectedThemes int
	}{
		{
			name:           "Valid JSON",
			resp:           `{"entities": ["Alice", "Wonderland"], "themes": ["Adventures"]}`,
			expectedEnts:   2,
			expectedThemes: 1,
		},
		{
			name:           "Garbage Response Fallback",
			resp:           "Not JSON at all",
			expectedEnts:   2, // Falls back to strings.Fields("test query")
			expectedThemes: 0,
		},
		{
			name:           "Empty JSON",
			resp:           `{}`,
			expectedEnts:   0,
			expectedThemes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockClient := &mockLLMClient{resp: tt.resp}
			extractor := NewDualKeyExtractor(&mockTaskRouter{client: mockClient})

			// Act
			keys, err := extractor.ExtractKeys(context.Background(), "test query")

			// Assert
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(keys.Entities) != tt.expectedEnts {
				t.Errorf("expected %d entities, got %d", tt.expectedEnts, len(keys.Entities))
			}
			if len(keys.Themes) != tt.expectedThemes {
				t.Errorf("expected %d themes, got %d", tt.expectedThemes, len(keys.Themes))
			}
		})
	}
}

package core

import "testing"

func TestTTSSafetyChunkLimit(t *testing.T) {
	tests := []struct {
		lang string
		want int
	}{
		{lang: "ko", want: 120},
		{lang: "ja", want: 120},
		{lang: "en", want: 300},
		{lang: "de", want: 300},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			if got := ttsSafetyChunkLimit(tt.lang); got != tt.want {
				t.Fatalf("ttsSafetyChunkLimit(%q) = %d, want %d", tt.lang, got, tt.want)
			}
		})
	}
}

func TestChunkTextHonorsJapaneseSafetyLimit(t *testing.T) {
	text := ""
	for i := 0; i < 121; i++ {
		text += "あ"
	}

	chunks := chunkText(text, ttsSafetyChunkLimit("ja"))
	if len(chunks) != 2 {
		t.Fatalf("chunkText returned %d chunks, want 2", len(chunks))
	}
	if got := len([]rune(chunks[0])); got > 120 {
		t.Fatalf("first chunk has %d runes, want at most 120", got)
	}
}

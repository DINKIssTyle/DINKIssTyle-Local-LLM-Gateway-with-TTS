package mcp

import (
	"crypto/sha256"
	"strings"
	"sync"
)

// Conversation turn vectors are derived data. Only turns from the caller's
// current snapshot are eligible, even if an old vector remains in memory.
var chatTurnVectors = struct {
	sync.Mutex
	generation uint64
	entries    map[[32]byte]queryEmbeddingValue
}{entries: make(map[[32]byte]queryEmbeddingValue)}

var chatTurnIndexSlot = make(chan struct{}, 1)

func resetChatTurnVectors() {
	chatTurnVectors.Lock()
	defer chatTurnVectors.Unlock()
	chatTurnVectors.generation++
	clear(chatTurnVectors.entries)
}

// MatchChatTurn returns the most relevant older turn, or -1. Candidates are
// newest first so ties prefer the user's latest statements.
func MatchChatTurn(scope, question string, candidates []string, useVectors bool) int {
	terms := automaticMemorySubjects(question)
	best, bestMatches := -1, 0
	if len(terms) == 0 || len(candidates) == 0 {
		return best
	}
	for i, candidate := range candidates {
		fields := splitSearchFields(strings.ToLower(candidate))
		matches := 0
		for _, term := range terms {
			eastAsian := hasEastAsianRune(term)
			for _, field := range fields {
				if field == term || (eastAsian && strings.Contains(field, term)) {
					matches++
					break
				}
			}
		}
		if matches > bestMatches {
			best, bestMatches = i, matches
		}
	}
	if !useVectors {
		return best
	}
	provider := getBufferedEmbeddingProvider()
	if provider.Build == nil && provider.BuildWithUsage == nil {
		return best
	}

	chatTurnVectors.Lock()
	generation := chatTurnVectors.generation
	vectors := make([]queryEmbeddingValue, len(candidates))
	var missing []pendingChatTurn
	for i, candidate := range candidates {
		key := sha256.Sum256([]byte(scope + "\x00" + candidate))
		vectors[i] = chatTurnVectors.entries[key]
		if len(vectors[i].vector) == 0 && len(missing) < 4 {
			missing = append(missing, pendingChatTurn{key: key, text: candidate})
		}
	}
	chatTurnVectors.Unlock()

	// Lexical hits require no additional embedding call on the request path.
	if best == -1 {
		for _, vector := range vectors {
			if len(vector.vector) > 0 {
				query, model := queryEmbedding(question)
				best = bestSemanticChatTurn(query, model, vectors)
				break
			}
		}
	}
	if len(missing) > 0 {
		warmChatTurnVectors(generation, missing)
	}
	return best
}

func bestSemanticChatTurn(query []float64, model string, vectors []queryEmbeddingValue) int {
	best, bestScore := -1, 0.55
	for i, candidate := range vectors {
		if candidate.model != model {
			continue
		}
		if score := cosineSimilarity(query, candidate.vector); score > bestScore {
			best, bestScore = i, score
		}
	}
	return best
}

type pendingChatTurn struct {
	key  [32]byte
	text string
}

func warmChatTurnVectors(generation uint64, items []pendingChatTurn) {
	select {
	case chatTurnIndexSlot <- struct{}{}:
	default:
		return
	}
	go func() {
		defer func() { <-chatTurnIndexSlot }()
		for _, item := range items {
			chatTurnVectors.Lock()
			stale := generation != chatTurnVectors.generation
			_, alreadyPresent := chatTurnVectors.entries[item.key]
			chatTurnVectors.Unlock()
			if stale {
				return
			}
			if alreadyPresent {
				continue
			}
			vector, model := buildBufferedEmbedding(item.text, BufferedEmbeddingUsageDocument)
			if len(vector) == 0 || model == webEmbeddingModel {
				continue
			}
			chatTurnVectors.Lock()
			if generation != chatTurnVectors.generation {
				chatTurnVectors.Unlock()
				return
			}
			if len(chatTurnVectors.entries) >= 512 {
				for evict := range chatTurnVectors.entries {
					delete(chatTurnVectors.entries, evict)
					break
				}
			}
			chatTurnVectors.entries[item.key] = queryEmbeddingValue{vector: vector, model: model}
			chatTurnVectors.Unlock()
		}
	}()
}

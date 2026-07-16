package chatharness

import "strings"

// IsValidResponseID reports whether id can be used with the Responses API's
// previous_response_id field. Other identifiers (for example tool call IDs)
// must never be persisted or forwarded as conversation state.
func IsValidResponseID(id string) bool {
	id = strings.TrimSpace(id)
	return strings.HasPrefix(id, "resp_") && len(id) > len("resp_")
}

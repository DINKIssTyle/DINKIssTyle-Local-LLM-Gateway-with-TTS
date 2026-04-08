package mcp

func init() {
	SetToolHooks(ToolHooks{
		Trace: func(source, stage, message string, details map[string]interface{}) {
			EmitTrace(source, stage, message, details)
		},
		SearchMemory: func(userID, query string) (string, error) {
			return SearchMemoryDB(userID, query)
		},
		ReadMemory: func(userID string, memoryID int64) (string, error) {
			return ReadMemoryDB(userID, memoryID)
		},
		DeleteMemory: func(userID string, memoryID int64) (string, error) {
			return DeleteMemoryDB(userID, memoryID)
		},
		BufferWebResult: func(userID, toolName, query, pageURL, title, content string) (string, error) {
			source := saveBufferedWebSource(userID, toolName, query, pageURL, title, content)
			return formatBufferedSourceHandle(source), nil
		},
		BufferWebFallback: func(userID, failedTool, target string, err error) string {
			return formatBufferedFallbackAfterToolError(userID, failedTool, target, err)
		},
		ReadBufferedSource: func(userID, sourceID, query string, maxChunks int) (string, error) {
			return readBufferedSource(userID, sourceID, query, maxChunks)
		},
		NormalizeSearchQuery:    defaultNormalizeSearchQuery,
		ReadPageTimeoutForURL:   defaultReadPageTimeoutForURL,
		ChallengeWaitIterations: defaultChallengeWaitIterations,
	})
}

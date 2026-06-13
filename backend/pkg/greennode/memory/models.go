// Package memory is the Go port of the GreenNode AgentBase Memory API client.
// Field names and JSON tags mirror the upstream OpenAPI spec (camelCase
// aliases). Optional fields use omitempty so partial requests serialize cleanly.
package memory

// Pagination is the common paginated envelope returned by list endpoints.
type Pagination struct {
	Page      int `json:"page,omitempty"`
	PageSize  int `json:"pageSize,omitempty"`
	TotalPage int `json:"totalPage,omitempty"`
	TotalItem int `json:"totalItem,omitempty"`
}

// LongTermMemoryStrategy configures automatic fact extraction for a memory.
type LongTermMemoryStrategy struct {
	Name                                  string `json:"name,omitempty"`
	Type                                  string `json:"type,omitempty"`
	CustomFactExtractionPrompt            string `json:"customFactExtractionPrompt,omitempty"`
	NamespaceTemplate                     string `json:"namespaceTemplate,omitempty"`
	EnableAutomaticMemoryRecordGeneration *bool  `json:"enableAutomaticMemoryRecordGeneration,omitempty"`
}

// CreateRequest is the body for creating a memory.
type CreateRequest struct {
	Name                    string                   `json:"name,omitempty"`
	Description             string                   `json:"description,omitempty"`
	EventExpiryDuration     *int                     `json:"eventExpiryDuration,omitempty"`
	LongTermMemoryStrategies []LongTermMemoryStrategy `json:"longTermMemoryStrategies,omitempty"`
}

// Entity is a memory resource.
type Entity struct {
	ID                  string `json:"id,omitempty"`
	Name                string `json:"name,omitempty"`
	Description         string `json:"description,omitempty"`
	EventExpiryDuration *int   `json:"eventExpiryDuration,omitempty"`
	PortalUserID        *int   `json:"portalUserId,omitempty"`
	Status              string `json:"status,omitempty"`
	CreatedAt           string `json:"createdAt,omitempty"`
	UpdatedAt           string `json:"updatedAt,omitempty"`
}

// ListResponse is the paginated list of memories.
type ListResponse struct {
	ListData []Entity `json:"listData"`
	Pagination
}

// Record is a stored memory record (a single fact / chunk).
type Record struct {
	ID        string   `json:"id,omitempty"`
	Memory    string   `json:"memory,omitempty"`
	Score     *float64 `json:"score,omitempty"`
	CreatedAt string   `json:"createdAt,omitempty"`
	UpdatedAt string   `json:"updatedAt,omitempty"`
}

// ListRecordsResponse is the paginated list of records.
type ListRecordsResponse struct {
	ListData []Record `json:"listData"`
	Pagination
}

// SearchRequest is the body for a semantic search over records.
type SearchRequest struct {
	Query          string   `json:"query,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	ScoreThreshold *float64 `json:"scoreThreshold,omitempty"`
}

// InsertDirectlyRequest inserts records without LLM extraction. Items are
// free-form (strings or objects) matching the upstream "memoryRecords" field.
type InsertDirectlyRequest struct {
	MemoryRecords []any `json:"memoryRecords,omitempty"`
}

// EventPayload is the content of an event.
type EventPayload struct {
	Type       string `json:"type,omitempty"`
	Role       string `json:"role,omitempty"`
	Message    string `json:"message,omitempty"`
	BinaryData string `json:"binaryData,omitempty"`
}

// CreateEventRequest is the body for appending an event to a session.
type CreateEventRequest struct {
	Payload        *EventPayload `json:"payload,omitempty"`
	EventTimestamp string        `json:"eventTimestamp,omitempty"`
}

// Event is a stored session event.
type Event struct {
	ID             string        `json:"id,omitempty"`
	ActorID        string        `json:"actorId,omitempty"`
	SessionID      string        `json:"sessionId,omitempty"`
	Payload        *EventPayload `json:"payload,omitempty"`
	Status         string        `json:"status,omitempty"`
	EventTimestamp string        `json:"eventTimestamp,omitempty"`
}

// ListEventsResponse is the paginated list of events.
type ListEventsResponse struct {
	ListData []Event `json:"listData"`
	Pagination
}

// Actor identifies a participant within a memory.
type Actor struct {
	MemoryID string `json:"memoryId,omitempty"`
	ActorID  string `json:"actorId,omitempty"`
	Status   string `json:"status,omitempty"`
}

// ListActorsResponse is the paginated list of actors.
type ListActorsResponse struct {
	ListData []Actor `json:"listData"`
	Pagination
}

// Session identifies a conversation session for an actor.
type Session struct {
	MemoryID  string `json:"memoryId,omitempty"`
	ActorID   string `json:"actorId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Status    string `json:"status,omitempty"`
}

// ListSessionsResponse is the paginated list of sessions.
type ListSessionsResponse struct {
	ListData []Session `json:"listData"`
	Pagination
}

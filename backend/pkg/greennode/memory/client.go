package memory

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/vngcloud/agentt/pkg/greennode"
)

// DefaultBaseURL is the GreenNode AgentBase Memory API base URL. Override via
// the GREENNODE_MEMORY_BASE_URL env var / config key.
const DefaultBaseURL = "https://agentbase.api.vngcloud.vn/memory"

// Client is a high-level client for the GreenNode AgentBase Memory API. Methods
// map 1:1 to the upstream operations used by this project.
type Client struct {
	rest *greennode.RESTClient
}

// Options configures a Memory Client.
type Options struct {
	// BaseURL overrides the Memory API base URL (defaults to config/DefaultBaseURL).
	BaseURL string
	// OAuth2TokenURL overrides the IAM token endpoint.
	OAuth2TokenURL string
	// Credentials overrides the IAM credentials (defaults to env/config resolution).
	Credentials *greennode.IAMCredentials
	// HTTPClient overrides the underlying *http.Client.
	HTTPClient *http.Client
}

// New builds a Memory Client. With zero Options it resolves base URL,
// token URL and credentials from environment variables / .greennode.json.
func New(opts Options) *Client {
	baseURL := opts.BaseURL
	if baseURL == "" {
		baseURL = greennode.GetConfigValue("GREENNODE_MEMORY_BASE_URL", DefaultBaseURL)
	}
	creds := greennode.NewIAMCredentials("", "")
	if opts.Credentials != nil {
		creds = *opts.Credentials
	}
	ts := greennode.NewTokenSource(creds, opts.OAuth2TokenURL, opts.HTTPClient)
	return &Client{rest: greennode.NewRESTClient(baseURL, ts, opts.HTTPClient)}
}

func pageQuery(page, size int) url.Values {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if size > 0 {
		q.Set("size", strconv.Itoa(size))
	}
	return q
}

// List returns memories (paginated).
func (c *Client) List(ctx context.Context, page, size int) (*ListResponse, error) {
	var out ListResponse
	if err := c.rest.Do(ctx, http.MethodGet, "/memories", pageQuery(page, size), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get returns a single memory by id.
func (c *Client) Get(ctx context.Context, id string) (*Entity, error) {
	var out Entity
	if err := c.rest.Do(ctx, http.MethodGet, "/memories/"+id, nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Create creates a memory.
func (c *Client) Create(ctx context.Context, req CreateRequest) (*Entity, error) {
	var out Entity
	if err := c.rest.Do(ctx, http.MethodPost, "/memories", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a memory by id.
func (c *Client) Delete(ctx context.Context, id string) error {
	return c.rest.Do(ctx, http.MethodDelete, "/memories/"+id, nil, nil, nil)
}

// SearchRecords performs a semantic search over a memory's records in the given
// namespace.
func (c *Client) SearchRecords(ctx context.Context, id, namespace string, req SearchRequest) (*ListRecordsResponse, error) {
	q := url.Values{}
	if namespace != "" {
		q.Set("namespace", namespace)
	}
	var records []Record
	if err := c.rest.Do(ctx, http.MethodPost, "/memories/"+id+"/memory-records:search", q, req, &records); err != nil {
		return nil, err
	}
	return &ListRecordsResponse{ListData: records}, nil
}

// InsertRecords inserts records directly (no LLM extraction) into a namespace.
func (c *Client) InsertRecords(ctx context.Context, id, namespace string, req InsertDirectlyRequest) error {
	q := url.Values{}
	if namespace != "" {
		q.Set("namespace", namespace)
	}
	return c.rest.Do(ctx, http.MethodPost, "/memories/"+id+"/memory-records:insert-directly", q, req, nil)
}

// ListRecords lists a memory's records (paginated).
func (c *Client) ListRecords(ctx context.Context, id string, page, size int) (*ListRecordsResponse, error) {
	var out ListRecordsResponse
	if err := c.rest.Do(ctx, http.MethodGet, "/memories/"+id+"/memory-records", pageQuery(page, size), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListActors lists actors within a memory (paginated).
func (c *Client) ListActors(ctx context.Context, id string, page, size int) (*ListActorsResponse, error) {
	var out ListActorsResponse
	if err := c.rest.Do(ctx, http.MethodGet, "/memories/"+id+"/actors", pageQuery(page, size), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListSessions lists sessions for an actor (paginated).
func (c *Client) ListSessions(ctx context.Context, id, actorID string, page, size int) (*ListSessionsResponse, error) {
	path := "/memories/" + id + "/actors/" + actorID + "/sessions"
	var out ListSessionsResponse
	if err := c.rest.Do(ctx, http.MethodGet, path, pageQuery(page, size), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateEvent appends an event to a session.
func (c *Client) CreateEvent(ctx context.Context, id, actorID, sessionID string, req CreateEventRequest) error {
	path := "/memories/" + id + "/actors/" + actorID + "/sessions/" + sessionID + "/events"
	return c.rest.Do(ctx, http.MethodPost, path, nil, req, nil)
}

// ListEvents lists events for a session (paginated).
func (c *Client) ListEvents(ctx context.Context, id, actorID, sessionID string, page, size int) (*ListEventsResponse, error) {
	path := "/memories/" + id + "/actors/" + actorID + "/sessions/" + sessionID + "/events"
	var out ListEventsResponse
	if err := c.rest.Do(ctx, http.MethodGet, path, pageQuery(page, size), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GenerateFromSession triggers long-term memory record generation from a
// session using the given strategy.
func (c *Client) GenerateFromSession(ctx context.Context, id, actorID, sessionID, strategyID string) error {
	q := url.Values{}
	q.Set("actorId", actorID)
	q.Set("sessionId", sessionID)
	if strategyID != "" {
		q.Set("longTermMemoryStrategyId", strategyID)
	}
	return c.rest.Do(ctx, http.MethodPost, "/memories/"+id+"/memory-records:generate-from-session", q, nil, nil)
}

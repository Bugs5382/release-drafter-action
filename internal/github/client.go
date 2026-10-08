package github

/*
ISC License

Copyright (c) 2026 Shane & Contributors

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
*/

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/go-github/v76/github"
	"github.com/rs/zerolog"
)

// defaultGraphQLPath is appended to the REST base URL's host to build the
// GraphQL endpoint, the way api.github.com does: the REST root is
// https://api.github.com and GraphQL lives at https://api.github.com/graphql.
const defaultGraphQLPath = "/graphql"

// maxAttempts bounds every retried call: the first try plus two retries.
const maxAttempts = 3

// Options configures a Client.
type Options struct {
	// RESTBaseURL is the REST API root; "" means https://api.github.com/.
	RESTBaseURL string
	// GraphQLURL is the GraphQL endpoint; "" means RESTBaseURL + "/graphql".
	GraphQLURL string
	// Token authenticates every call. It is never logged.
	Token string
	// Transport is the underlying RoundTripper; nil means
	// http.DefaultTransport. Tests point a Client at an httptest.Server
	// through RESTBaseURL/GraphQLURL instead of Transport, which stays free
	// for exercising the retry behavior itself (a RoundTripper that fails a
	// set number of times before succeeding).
	Transport http.RoundTripper
	// Sleep backs off between retries; nil means time.Sleep. Tests pass a
	// no-op so a retry test does not actually wait.
	Sleep func(time.Duration)
	Log   *zerolog.Logger
}

// Client is a thin wrapper over go-github (REST) and a hand-rolled GraphQL
// caller, both sharing one retrying http.Client so every request, REST or
// GraphQL, gets the same backoff on 5xx and secondary rate limits.
type Client struct {
	rest       *github.Client
	graphQLURL string
	token      string
	http       *http.Client
	log        *zerolog.Logger
}

// retryTransport retries the wrapped RoundTripper on a transport error, a
// 429 (including GitHub's secondary rate limit) and any 5xx, up to
// maxAttempts times with linear backoff. It never retries a request whose
// body cannot be re-sent (GetBody is nil and Body is non-nil), since the
// body would already be drained.
type retryTransport struct {
	base  http.RoundTripper
	sleep func(time.Duration)
	log   *zerolog.Logger
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		cur := req
		if attempt > 1 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("github: rewinding the request body for a retry: %w", err)
			}
			clone := req.Clone(req.Context())
			clone.Body = body
			cur = clone
		}
		start := time.Now()
		resp, err := t.base.RoundTrip(cur)
		switch {
		case err != nil:
			lastErr = err
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			_ = resp.Body.Close()
		default:
			t.log.Debug().Str("method", req.Method).Str("path", req.URL.Path).Int("status", resp.StatusCode).
				Dur("duration", time.Since(start)).Int("attempt", attempt).Msg("github: request")
			return resp, nil
		}
		t.log.Warn().Str("method", req.Method).Str("path", req.URL.Path).Int("attempt", attempt).Err(lastErr).
			Dur("duration", time.Since(start)).Msg("github: request failed, retrying")
		if attempt < maxAttempts {
			t.sleep(time.Duration(attempt) * time.Second)
		}
	}
	return nil, fmt.Errorf("github: %s %s failed after %d attempts: %w", req.Method, req.URL.Path, maxAttempts, lastErr)
}

// New builds a Client. The token is set once here and never logged.
func New(o Options) *Client {
	sleep := o.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	log := o.Log
	if log == nil {
		nop := zerolog.Nop()
		log = &nop
	}
	base := o.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	httpClient := &http.Client{Transport: &retryTransport{base: base, sleep: sleep, log: log}}

	rest := github.NewClient(httpClient).WithAuthToken(o.Token)
	if o.RESTBaseURL != "" {
		u, err := url.Parse(strings.TrimSuffix(o.RESTBaseURL, "/") + "/")
		if err == nil {
			rest.BaseURL = u
		}
	}
	graphQLURL := o.GraphQLURL
	if graphQLURL == "" {
		graphQLURL = strings.TrimSuffix(rest.BaseURL.String(), "/") + defaultGraphQLPath
	}
	return &Client{rest: rest, graphQLURL: graphQLURL, token: o.Token, http: httpClient, log: log}
}

// graphQLRequest is the envelope every GraphQL call sends.
type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphQLError is one entry of a GraphQL response's "errors" array.
type graphQLError struct {
	Message string `json:"message"`
}

// GraphQLError is every error a GraphQL response reported. The query still
// ran, so partial data may be present in out; callers that do not need it
// can ignore anything already decoded.
type GraphQLError struct {
	Messages []string
}

func (e *GraphQLError) Error() string {
	return "github: graphql: " + strings.Join(e.Messages, "; ")
}

// doGraphQL executes query with variables and decodes the "data" field into
// out. A query that returns a non-empty "errors" array fails with a
// *GraphQLError even when "data" also came back.
func (c *Client) doGraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	body, err := json.Marshal(graphQLRequest{Query: query, Variables: variables})
	if err != nil {
		return fmt.Errorf("github: encoding the graphql request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.graphQLURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("github: building the graphql request: %w", err)
	}
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "release-drafter-action")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("github: graphql request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("github: graphql request returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []graphQLError  `json:"errors"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(&envelope); err != nil {
		return fmt.Errorf("github: decoding the graphql response: %w", err)
	}
	if len(envelope.Data) > 0 && !bytes.Equal(envelope.Data, []byte("null")) {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return fmt.Errorf("github: decoding the graphql data: %w", err)
		}
	}
	if len(envelope.Errors) > 0 {
		messages := make([]string, len(envelope.Errors))
		for i, e := range envelope.Errors {
			messages[i] = e.Message
		}
		return &GraphQLError{Messages: messages}
	}
	return nil
}

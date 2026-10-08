package action

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

// Env is the subset of the GitHub Actions runtime Run reads: the event that
// picks the mode, the token and repository, the checkout and API locations,
// and where to append $GITHUB_OUTPUT. cmd/action builds one from os.Getenv;
// tests build one by hand, pointed at a fake server.
type Env struct {
	// EventName is GITHUB_EVENT_NAME; it picks the mode.
	EventName string
	// EventPath is GITHUB_EVENT_PATH, read in autolabel mode for the
	// pull_request event payload.
	EventPath string
	// Token is GITHUB_TOKEN, never logged.
	Token string
	// Repository is GITHUB_REPOSITORY, "owner/repo".
	Repository string
	// Workspace is GITHUB_WORKSPACE, the checked-out repository, read for
	// the config-name file.
	Workspace string
	// Ref is GITHUB_REF, the default commitish when neither the config nor
	// the commitish input sets one.
	Ref string
	// ServerURL is GITHUB_SERVER_URL, used to build author and compare
	// links; "" becomes https://github.com.
	ServerURL string
	// APIURL is GITHUB_API_URL; "" means https://api.github.com.
	APIURL string
	// GraphQLURL is GITHUB_GRAPHQL_URL; "" is derived from APIURL.
	GraphQLURL string
	// OutputPath is GITHUB_OUTPUT; "" skips writing outputs (a local run
	// with no file to append to).
	OutputPath string
}

// LoadEnv reads Env from getenv (os.Getenv in production), applying the one
// default GitHub Actions itself guarantees but a local run might not set.
func LoadEnv(getenv func(string) string) Env {
	e := Env{
		EventName:  getenv("GITHUB_EVENT_NAME"),
		EventPath:  getenv("GITHUB_EVENT_PATH"),
		Token:      getenv("GITHUB_TOKEN"),
		Repository: getenv("GITHUB_REPOSITORY"),
		Workspace:  getenv("GITHUB_WORKSPACE"),
		Ref:        getenv("GITHUB_REF"),
		ServerURL:  getenv("GITHUB_SERVER_URL"),
		APIURL:     getenv("GITHUB_API_URL"),
		GraphQLURL: getenv("GITHUB_GRAPHQL_URL"),
		OutputPath: getenv("GITHUB_OUTPUT"),
	}
	if e.ServerURL == "" {
		e.ServerURL = "https://github.com"
	}
	return e
}

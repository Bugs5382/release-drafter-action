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

import "github.com/Bugs5382/release-drafter-action/internal/model"

// pullRequestFieldsFragment is release-drafter v7's PullRequestFields
// fragment: the fields the drafter and the recovery query both need. The
// three @include directives are driven by whether the change-template
// actually uses $BODY, $URL, $BASE_REF_NAME or $HEAD_REF_NAME, the same way
// v7 only asks GitHub for what it will render.
const pullRequestFieldsFragment = `
fragment PullRequestFields on PullRequest {
  __typename
  title
  number
  url @include(if: $withPullRequestURL)
  body @include(if: $withPullRequestBody)
  author {
    __typename
    login
    url
  }
  baseRepository {
    __typename
    nameWithOwner
  }
  mergedAt
  isCrossRepository
  labels(first: 100) {
    __typename
    nodes {
      __typename
      name
    }
  }
  merged
  baseRefName @include(if: $withBaseRefName)
  headRefName @include(if: $withHeadRefName)
}
`

// findCommitsInComparisonQuery walks the commits between two refs, each
// with its associated pull requests.
const findCommitsInComparisonQuery = pullRequestFieldsFragment + `
query findCommitsInComparison(
  $name: String!
  $owner: String!
  $baseRef: String!
  $headRef: String!
  $withPullRequestBody: Boolean!
  $withPullRequestURL: Boolean!
  $cursor: String
  $withBaseRefName: Boolean!
  $withHeadRefName: Boolean!
  $pullRequestLimit: Int!
  $historyLimit: Int!
) {
  repository(name: $name, owner: $owner) {
    ref(qualifiedName: $baseRef) {
      compare(headRef: $headRef) {
        commits(first: $historyLimit, after: $cursor) {
          pageInfo {
            hasNextPage
            endCursor
          }
          nodes {
            id
            oid
            committedDate
            message
            author {
              name
              user {
                login
              }
            }
            authors(first: 100) {
              nodes {
                name
                user {
                  login
                }
              }
            }
            associatedPullRequests(first: $pullRequestLimit) {
              nodes {
                ...PullRequestFields
              }
            }
          }
        }
      }
    }
  }
}
`

// findCommitsSinceQuery is this action's own first-release walk: the
// history reachable from the target commit, trimmed by since (a
// GitTimestamp) when one is given. v7 has no equivalent; a first release
// has no lastRelease tag to compare against, so there is nothing to pass as
// baseRef.
const findCommitsSinceQuery = pullRequestFieldsFragment + `
query findCommitsSince(
  $name: String!
  $owner: String!
  $target: String!
  $since: GitTimestamp
  $withPullRequestBody: Boolean!
  $withPullRequestURL: Boolean!
  $cursor: String
  $withBaseRefName: Boolean!
  $withHeadRefName: Boolean!
  $pullRequestLimit: Int!
  $historyLimit: Int!
) {
  repository(name: $name, owner: $owner) {
    object(expression: $target) {
      ... on Commit {
        history(since: $since, first: $historyLimit, after: $cursor) {
          pageInfo {
            hasNextPage
            endCursor
          }
          nodes {
            id
            oid
            committedDate
            message
            author {
              name
              user {
                login
              }
            }
            authors(first: 100) {
              nodes {
                name
                user {
                  login
                }
              }
            }
            associatedPullRequests(first: $pullRequestLimit) {
              nodes {
                ...PullRequestFields
              }
            }
          }
        }
      }
    }
  }
}
`

// findRecentMergedPullRequestsQuery recovers pull requests GitHub's
// associatedPullRequests index has not caught up with yet: it reads the
// most recently merged pull requests directly and the caller matches their
// mergeCommit.oid against the commit range.
const findRecentMergedPullRequestsQuery = pullRequestFieldsFragment + `
query findRecentMergedPullRequests(
  $name: String!
  $owner: String!
  $baseRefName: String
  $limit: Int!
  $withPullRequestBody: Boolean!
  $withPullRequestURL: Boolean!
  $withBaseRefName: Boolean!
  $withHeadRefName: Boolean!
) {
  repository(name: $name, owner: $owner) {
    pullRequests(
      states: [MERGED]
      baseRefName: $baseRefName
      orderBy: { field: UPDATED_AT, direction: DESC }
      first: $limit
    ) {
      nodes {
        ...PullRequestFields
        mergeCommit {
          oid
        }
      }
    }
  }
}
`

// resolveCommitishQuery resolves a git expression (a tag, SHA or ref) to a
// commit SHA.
const resolveCommitishQuery = `
query resolveCommitish($name: String!, $owner: String!, $expression: String!) {
  repository(name: $name, owner: $owner) {
    object(expression: $expression) {
      __typename
      oid
    }
  }
}
`

// gqlActor is a PullRequestFields author or a commit author's linked user.
type gqlActor struct {
	Typename string `json:"__typename"`
	Login    string `json:"login"`
	URL      string `json:"url"`
}

type gqlLabels struct {
	Nodes []struct {
		Name string `json:"name"`
	} `json:"nodes"`
}

// gqlPullRequest mirrors the PullRequestFields fragment, plus the
// mergeCommit field the recovery query also asks for.
type gqlPullRequest struct {
	Title          string    `json:"title"`
	Number         int       `json:"number"`
	URL            *string   `json:"url"`
	Body           *string   `json:"body"`
	Author         *gqlActor `json:"author"`
	BaseRepository *struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"baseRepository"`
	MergedAt          *string   `json:"mergedAt"`
	IsCrossRepository bool      `json:"isCrossRepository"`
	Labels            gqlLabels `json:"labels"`
	Merged            bool      `json:"merged"`
	BaseRefName       *string   `json:"baseRefName"`
	HeadRefName       *string   `json:"headRefName"`
	MergeCommit       *struct {
		Oid string `json:"oid"`
	} `json:"mergeCommit"`
}

func (g gqlPullRequest) toModel() model.PullRequest {
	out := model.PullRequest{
		Number:            g.Number,
		Title:             g.Title,
		URL:               g.URL,
		Body:              g.Body,
		MergedAt:          g.MergedAt,
		IsCrossRepository: g.IsCrossRepository,
		Merged:            g.Merged,
		BaseRefName:       g.BaseRefName,
		HeadRefName:       g.HeadRefName,
	}
	if g.BaseRepository != nil {
		out.BaseRepository = g.BaseRepository.NameWithOwner
	}
	if g.Author != nil {
		out.Author = &model.Actor{Typename: g.Author.Typename, Login: g.Author.Login, URL: g.Author.URL}
	}
	for _, l := range g.Labels.Nodes {
		out.Labels = append(out.Labels, l.Name)
	}
	if g.MergeCommit != nil {
		out.MergeCommitOID = g.MergeCommit.Oid
	}
	return out
}

// gqlCommitAuthor mirrors a commit author: a free-text name, and the
// GitHub user it resolves to when there is one.
type gqlCommitAuthor struct {
	Name string `json:"name"`
	User *struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (g gqlCommitAuthor) toModel() model.CommitAuthor {
	out := model.CommitAuthor{Name: g.Name}
	if g.User != nil {
		out.Login = g.User.Login
	}
	return out
}

type gqlCommitNode struct {
	Oid           string           `json:"oid"`
	CommittedDate string           `json:"committedDate"`
	Message       string           `json:"message"`
	Author        *gqlCommitAuthor `json:"author"`
	Authors       struct {
		Nodes []gqlCommitAuthor `json:"nodes"`
	} `json:"authors"`
	AssociatedPullRequests struct {
		Nodes []gqlPullRequest `json:"nodes"`
	} `json:"associatedPullRequests"`
}

func (g gqlCommitNode) toModel() model.Commit {
	out := model.Commit{OID: g.Oid, CommittedDate: g.CommittedDate, Message: g.Message}
	if g.Author != nil {
		author := g.Author.toModel()
		out.Author = &author
	}
	for _, a := range g.Authors.Nodes {
		out.Authors = append(out.Authors, a.toModel())
	}
	for _, pr := range g.AssociatedPullRequests.Nodes {
		out.PullRequests = append(out.PullRequests, pr.toModel())
	}
	return out
}

type gqlPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

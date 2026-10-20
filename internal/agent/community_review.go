package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// Only the explicitly submitted recipe and normalized image leave the service.
// This adapter has no database access and cannot publish, award points or call tools.
type CommunitySubmission struct {
	Title       string          `json:"title"`
	Story       string          `json:"story"`
	Servings    int             `json:"servings"`
	Minutes     int             `json:"minutes"`
	Tags        []string        `json:"tags"`
	Ingredients json.RawMessage `json:"ingredients"`
	Steps       []string        `json:"steps"`
	Cover       []byte          `json:"-"`
}
type ReviewIssue struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}
type CommunityReview struct {
	Decision     string        `json:"decision"`
	ImageChecked bool          `json:"image_checked"`
	Issues       []ReviewIssue `json:"issues"`
}
type CommunityReviewer interface {
	Review(context.Context, CommunitySubmission, CallGuard) (CommunityReview, error)
}
type CommunityReviewModel struct {
	Endpoint, Name, Key string
}

// Dedicated settings are an atomic credential triple. Partial overrides never
// send another provider's key to a different endpoint.
func CommunityReviewFromEnv(withCover bool) CommunityReviewModel {
	endpoint, name, key := os.Getenv("COMMUNITY_REVIEW_MODEL_ENDPOINT"), os.Getenv("COMMUNITY_REVIEW_MODEL_NAME"), os.Getenv("COMMUNITY_REVIEW_MODEL_API_KEY")
	if endpoint != "" || name != "" || key != "" {
		return CommunityReviewModel{Endpoint: endpoint, Name: name, Key: key}
	}
	if withCover {
		v := VisionFromEnv()
		return CommunityReviewModel{Endpoint: v.Endpoint, Name: v.Name, Key: v.Key}
	}
	return CommunityReviewModel{Endpoint: os.Getenv("MODEL_ENDPOINT"), Name: os.Getenv("MODEL_NAME"), Key: os.Getenv("MODEL_API_KEY")}
}
func (m CommunityReviewModel) Configured() bool {
	return m.Endpoint != "" && m.Name != "" && m.Key != ""
}

func ParseCommunityReview(content string, withCover bool) (CommunityReview, error) {
	var wire struct {
		Decision     string        `json:"decision"`
		ImageChecked *bool         `json:"image_checked"`
		Issues       []ReviewIssue `json:"issues"`
	}
	var v CommunityReview
	dec := json.NewDecoder(strings.NewReader(content))
	dec.DisallowUnknownFields()
	if len(content) > 8192 || dec.Decode(&wire) != nil || dec.Decode(new(any)) != io.EOF || wire.ImageChecked == nil {
		return v, errors.New("invalid community review output")
	}
	v = CommunityReview{Decision: wire.Decision, ImageChecked: *wire.ImageChecked, Issues: wire.Issues}
	if v.Decision != "approved" && v.Decision != "rejected" || withCover != v.ImageChecked || v.Issues == nil || len(v.Issues) > 6 || v.Decision == "approved" && len(v.Issues) != 0 || v.Decision == "rejected" && len(v.Issues) == 0 {
		return v, errors.New("incomplete community review")
	}
	for _, issue := range v.Issues {
		if issue.Field != "title" && issue.Field != "story" && issue.Field != "ingredients" && issue.Field != "steps" && issue.Field != "cover" || issue.Field == "cover" && !withCover || utf8.RuneCountInString(strings.TrimSpace(issue.Reason)) < 1 || utf8.RuneCountInString(issue.Reason) > 200 {
			return v, errors.New("invalid community review issue")
		}
	}
	return v, nil
}

func (m CommunityReviewModel) Review(ctx context.Context, submission CommunitySubmission, guard CallGuard) (CommunityReview, error) {
	if !m.Configured() || guard == nil {
		return CommunityReview{}, errors.New("community review requires configured model and allowance protection")
	}
	if len(submission.Cover) > 2<<20 {
		return CommunityReview{}, errors.New("review image too large")
	}
	content, err := callPython(ctx, pythonRequest{Task: "review", Endpoint: m.Endpoint, Model: m.Name, Key: m.Key, Context: submission, Data: submission.Cover}, guard, nil)
	if err != nil {
		return CommunityReview{}, err
	}
	return ParseCommunityReview(content, len(submission.Cover) > 0)
}

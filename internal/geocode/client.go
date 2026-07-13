// Package geocode wraps the OpenCage Geocoding API. It resolves free-text
// addresses to coordinates only — administrative breakdown (province/city/
// district) comes from the internal/wilayah reference tables, not from
// OpenCage's inconsistent component tagging.
package geocode

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://api.opencagedata.com/geocode/v1/json"

// ErrNoResults is returned by Geocode when OpenCage finds no match for the
// given address. Search does not use this — an empty query result there is
// a valid "nothing yet" outcome (e.g. autocomplete), not an error.
var ErrNoResults = errors.New("no geocoding results found")

type Result struct {
	Formatted  string
	Latitude   float64
	Longitude  float64
	Confidence int
}

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Geocode resolves a single address to its best-matching coordinates.
// Returns ErrNoResults if OpenCage finds nothing.
func (c *Client) Geocode(ctx context.Context, address string) (*Result, error) {
	params := url.Values{
		"key":            {c.apiKey},
		"q":              {address},
		"limit":          {"1"},
		"no_annotations": {"1"},
	}

	results, err := c.request(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, ErrNoResults
	}

	return &results[0], nil
}

// Search returns up to limit candidate matches for query, biased to
// Indonesia. Returns (nil, nil) when query is too short or nothing matches
// — this is a valid empty result, not an error (used for autocomplete).
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	q := strings.TrimSpace(query)
	if len(q) < 3 {
		return nil, nil
	}

	if limit < 1 {
		limit = 1
	} else if limit > 10 {
		limit = 10
	}

	params := url.Values{
		"key":            {c.apiKey},
		"q":              {q},
		"limit":          {fmt.Sprintf("%d", limit)},
		"no_annotations": {"1"},
		"countrycode":    {"id"},
		"language":       {"id"},
		"min_confidence": {"3"},
		"bounds":         {"94.0,-11.0,141.0,6.0"},
	}

	results, err := c.request(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	return results, nil
}

type openCageResponse struct {
	Results []struct {
		Formatted  string `json:"formatted"`
		Confidence int    `json:"confidence"`
		Geometry   struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"geometry"`
	} `json:"results"`
}

func (c *Client) request(ctx context.Context, params url.Values) ([]Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("error in building geocode request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error in performing geocode request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code from opencage: %d", resp.StatusCode)
	}

	var parsed openCageResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("error in decoding geocode response: %w", err)
	}

	results := make([]Result, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		results = append(results, Result{
			Formatted:  r.Formatted,
			Latitude:   r.Geometry.Lat,
			Longitude:  r.Geometry.Lng,
			Confidence: r.Confidence,
		})
	}

	return results, nil
}

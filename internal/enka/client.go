package enka

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

const userAgent = "Gscouter/0.1 (+https://github.com/cenoda/gscouter)"

var uidPattern = regexp.MustCompile(`^[1-9][0-9]{8,9}$`)

// Errors returned by Fetch, matching Enka's documented status codes.
var (
	ErrInvalidUID  = errors.New("invalid UID")
	ErrNotFound    = errors.New("player not found")
	ErrMaintenance = errors.New("the game is under maintenance")
	ErrRateLimited = errors.New("rate limited by Enka.Network, try again shortly")
	ErrUnavailable = errors.New("Enka.Network is unavailable")
)

type Client struct {
	HTTP    *http.Client
	BaseURL string // defaults to https://enka.network
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second}}
}

// Fetch returns the raw /api/uid/<uid> response.
func (c *Client) Fetch(ctx context.Context, uid string) ([]byte, error) {
	if !uidPattern.MatchString(uid) {
		return nil, ErrInvalidUID
	}
	return c.get(ctx, "/api/uid/"+uid+"/")
}

// ownerPattern bounds an Enka account's username and a game account's
// hash, which go into a URL path.
var ownerPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// FetchBuilds returns the raw response listing the builds an Enka
// account saved for one of its game accounts:
// /api/profile/<username>/hoyos/<hash>/builds/.
func (c *Client) FetchBuilds(ctx context.Context, owner Owner) ([]byte, error) {
	if !ownerPattern.MatchString(owner.Username) || !ownerPattern.MatchString(owner.Hash) {
		return nil, ErrNotFound
	}
	return c.get(ctx, "/api/profile/"+url.PathEscape(owner.Username)+"/hoyos/"+url.PathEscape(owner.Hash)+"/builds/")
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	base := c.BaseURL
	if base == "" {
		base = "https://enka.network"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusBadRequest:
		return nil, ErrInvalidUID
	case http.StatusNotFound:
		return nil, ErrNotFound
	case http.StatusFailedDependency:
		return nil, ErrMaintenance
	case http.StatusTooManyRequests:
		return nil, ErrRateLimited
	default:
		return nil, fmt.Errorf("%w: status %d", ErrUnavailable, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 10<<20))
}

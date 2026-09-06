// Package mailclaw talks to the MailClaw HTTP API without invoking its CLI.
package mailclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const maxJSONSize = 64 << 20

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

func New(host, token string) (*Client, error) {
	base, err := url.Parse(strings.TrimSpace(host))
	if err != nil || base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.ForceQuery || base.Fragment != "" || base.Opaque != "" {
		return nil, errors.New("mailclaw: host must be a base URL without credentials, query, or fragment")
	}
	ip := net.ParseIP(base.Hostname())
	loopback := strings.EqualFold(base.Hostname(), "localhost") || (ip != nil && ip.IsLoopback())
	if base.Scheme != "https" && !(base.Scheme == "http" && loopback) {
		return nil, errors.New("mailclaw: HTTPS is required (HTTP is allowed only on loopback)")
	}
	token = strings.TrimSpace(token)
	if token == "" || strings.IndexFunc(token, unicode.IsSpace) >= 0 || strings.IndexFunc(token, unicode.IsControl) >= 0 {
		return nil, errors.New("mailclaw: a non-empty API token without whitespace is required")
	}
	base.Path = strings.TrimRight(base.Path, "/")
	base.RawPath = ""
	return &Client{base: base, token: token, http: &http.Client{
		Timeout: 60 * time.Second,
		// Never forward credentials or replay a mutation through a redirect.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// EmailPath rejects ambiguous path segments before they can select another route.
func EmailPath(id string) (string, error) {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, "/\\?#%") || strings.IndexFunc(id, unicode.IsControl) >= 0 || strings.TrimSpace(id) != id {
		return "", errors.New("mailclaw: invalid email or attachment ID")
	}
	return "/api/emails/" + id, nil
}

func (c *Client) request(ctx context.Context, method, path string, query url.Values, payload any) (*http.Response, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, errors.New("mailclaw: cannot encode request")
		}
		body = bytes.NewReader(data)
	}
	u := *c.base
	u.Path += path
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, errors.New("mailclaw: cannot build request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Transport errors can include URLs or reflected headers: do not print them.
		return nil, errors.New("mailclaw: HTTP request failed (check host, TLS, network, or timeout)")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("mailclaw: HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

// Do returns the API's data object, preserving string IDs and upstream field names.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, payload any) (json.RawMessage, error) {
	resp, err := c.request(ctx, method, path, query, payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxJSONSize+1))
	if err != nil {
		return nil, errors.New("mailclaw: cannot read API response")
	}
	if len(data) > maxJSONSize {
		return nil, errors.New("mailclaw: JSON response exceeds 64 MiB; reduce --limit")
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
	}
	if json.Unmarshal(data, &envelope) != nil {
		return nil, errors.New("mailclaw: invalid JSON response")
	}
	if !envelope.Success {
		return nil, errors.New("mailclaw: API rejected the request")
	}
	if len(envelope.Data) == 0 || bytes.Equal(envelope.Data, []byte("null")) {
		return nil, errors.New("mailclaw: API response is missing data")
	}
	return envelope.Data, nil
}

// Download ignores remote filenames. A complete private temporary file is linked
// into place without overwriting an existing file (including a symlink).
func (c *Client) Download(ctx context.Context, emailID, attachmentID, destination string) (int64, error) {
	path, err := EmailPath(emailID)
	if err != nil {
		return 0, err
	}
	if _, err := EmailPath(attachmentID); err != nil {
		return 0, err
	}
	if destination == "" {
		return 0, errors.New("mailclaw: download requires an output path")
	}
	if _, err := os.Lstat(destination); err == nil {
		return 0, errors.New("mailclaw: output already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	resp, err := c.request(ctx, http.MethodGet, path+"/attachments/"+attachmentID, nil, nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	file, err := os.CreateTemp(filepath.Dir(destination), ".email-cli-download-*")
	if err != nil {
		return 0, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	n, err := io.Copy(file, resp.Body)
	if err != nil {
		return 0, errors.New("mailclaw: incomplete attachment download")
	}
	if resp.ContentLength >= 0 && n != resp.ContentLength {
		return 0, errors.New("mailclaw: attachment size mismatch")
	}
	if err := file.Close(); err != nil {
		return 0, err
	}
	if err := os.Link(file.Name(), destination); err != nil {
		return 0, err
	}
	return n, nil
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type client struct {
	base  string
	token string
	http  *http.Client
}

type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string { return fmt.Sprintf("archive: %d: %s", e.Status, e.Message) }

func notFound(err error) bool {
	e, ok := err.(*apiError)
	return ok && e.Status == http.StatusNotFound
}

func (c *client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return &apiError{Status: resp.StatusCode, Message: errorMessage(b)}
	}
	if out == nil || len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, out)
}

func errorMessage(b []byte) string {
	var e struct {
		Error  string `json:"error"`
		Type   string `json:"type"`
		Errors []struct {
			Path    string `json:"path"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(b, &e) != nil {
		return strings.TrimSpace(string(b))
	}
	if e.Error != "" {
		return e.Error
	}
	parts := []string{}
	for _, fe := range e.Errors {
		parts = append(parts, fe.Path+": "+fe.Message)
	}
	return "invalid " + e.Type + ": " + strings.Join(parts, "; ")
}

func esc(s string) string { return url.PathEscape(s) }

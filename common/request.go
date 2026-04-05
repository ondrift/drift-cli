package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

func NewAuthenticatedRequest(method, url string, body io.Reader) (*http.Request, error) {
	token, _, err := GetTokenFromSession()
	if err != nil {
		return nil, fmt.Errorf("failed to get token from session: %w", err)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	if slice := GetActiveSlice(); slice != "" {
		req.Header.Set("X-Slice", slice)
	}

	return req, nil
}

// DoRequest executes an authenticated HTTP request. If the server returns 401
// (token expired), it automatically refreshes the JWT using the stored refresh
// token and retries once. The body parameter is read, buffered, and replayed
// on retry so callers don't need to worry about re-seekable readers.
func DoRequest(method, url string, body io.Reader) (*http.Response, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}

	req, err := NewAuthenticatedRequest(method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	resp.Body.Close()

	// Attempt token refresh.
	if err := refreshAccessToken(); err != nil {
		return nil, fmt.Errorf("session expired — run 'drift account login' to re-authenticate")
	}

	// Retry with the new token.
	req, err = NewAuthenticatedRequest(method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	return httpClient.Do(req)
}

// refreshAccessToken uses the stored refresh token to obtain a new access
// token from the API, persisting both new tokens to the session file.
func refreshAccessToken() error {
	_, refreshToken, err := GetTokenFromSession()
	if err != nil {
		return err
	}
	if refreshToken == "" {
		return fmt.Errorf("no refresh token stored")
	}

	body, _ := json.Marshal(map[string]string{
		"refresh_token": refreshToken,
	})

	resp, err := httpClient.Post(APIBaseURL+"/refresh", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("refresh returned %d", resp.StatusCode)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse refresh response: %w", err)
	}

	return SaveSession(result.AccessToken, result.RefreshToken)
}

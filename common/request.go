package common

import (
	"fmt"
	"io"
	"net/http"
)

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
	return req, nil
}

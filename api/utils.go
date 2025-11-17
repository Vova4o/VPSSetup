package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	timeoutShort  = 30 * time.Second
	timeoutMedium = 2 * time.Minute
	timeoutLong   = 10 * time.Minute
)

// parseJSON parses JSON from request body
func parseJSON(r *http.Request, v interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}
	defer r.Body.Close()

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

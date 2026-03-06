package core

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BreachChecker is the interface for checking passwords against breach databases.
type BreachChecker interface {
	IsBreached(password string) (bool, error)
}

// HIBPChecker checks passwords against the Have I Been Pwned API using k-anonymity.
// Only the first 5 characters of the SHA-1 hash are sent to the API.
type HIBPChecker struct {
	Client *http.Client
}

// NewHIBPChecker creates a new HIBP checker with sensible defaults.
func NewHIBPChecker() *HIBPChecker {
	return &HIBPChecker{
		Client: &http.Client{Timeout: 5 * time.Second},
	}
}

// IsBreached checks if the password appears in the HIBP database.
func (h *HIBPChecker) IsBreached(password string) (bool, error) {
	hash := fmt.Sprintf("%X", sha1.Sum([]byte(password)))
	prefix := hash[:5]
	suffix := hash[5:]

	url := "https://api.pwnedpasswords.com/range/" + prefix
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf(MsgErrCreatingRequest, err)
	}
	req.Header.Set("User-Agent", "PassForge-PasswordChecker")
	req.Header.Set("Add-Padding", "true")

	resp, err := h.Client.Do(req)
	if err != nil {
		return false, fmt.Errorf(MsgErrHIBPRequest, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf(MsgErrHIBPStatus, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, fmt.Errorf(MsgErrHIBPRead, err)
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], suffix) {
			return true, nil
		}
	}

	return false, nil
}

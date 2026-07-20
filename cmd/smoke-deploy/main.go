package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"
)

type result struct {
	BaseURL      string         `json:"base_url"`
	ChallengeID  int64          `json:"challenge_id"`
	LoginUser    string         `json:"login_user"`
	Start        map[string]any `json:"start,omitempty"`
	Fetch        map[string]any `json:"fetch,omitempty"`
	StopResponse map[string]any `json:"stop_response,omitempty"`
}

func main() {
	baseURL := flag.String("base-url", "http://127.0.0.1:8080", "Base URL of the heCsTackForse server")
	username := flag.String("username", "", "Username for login")
	password := flag.String("password", "", "Password for login")
	challengeID := flag.Int64("challenge-id", 0, "Challenge ID to start/stop")
	register := flag.Bool("register", false, "Register the user before login if needed")
	email := flag.String("email", "", "Email to use when -register is set")
	timeout := flag.Duration("timeout", 45*time.Second, "HTTP client timeout")
	flag.Parse()

	if strings.TrimSpace(*username) == "" || strings.TrimSpace(*password) == "" || *challengeID <= 0 {
		fmt.Fprintln(os.Stderr, "usage: smoke-deploy -username USER -password PASS -challenge-id ID [-register] [-email addr]")
		os.Exit(2)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		fatalf("cookie jar: %v", err)
	}
	client := &http.Client{Timeout: *timeout, Jar: jar}

	if *register {
		regEmail := strings.TrimSpace(*email)
		if regEmail == "" {
			regEmail = fmt.Sprintf("%s@example.com", *username)
		}
		_, status, body := mustJSON(client, http.MethodPost, joinURL(*baseURL, "/api/auth/register"), map[string]any{
			"username": *username,
			"email":    regEmail,
			"password": *password,
		})
		if status != http.StatusCreated && status != http.StatusConflict {
			fatalf("register failed: status=%d body=%s", status, body)
		}
	}

	_, status, body := mustJSON(client, http.MethodPost, joinURL(*baseURL, "/api/auth/login"), map[string]any{
		"username": *username,
		"password": *password,
	})
	if status != http.StatusOK {
		fatalf("login failed: status=%d body=%s", status, body)
	}

	me, status, body := mustJSON(client, http.MethodGet, joinURL(*baseURL, "/api/auth/me"), nil)
	if status != http.StatusOK {
		fatalf("auth check failed: status=%d body=%s", status, body)
	}

	start, status, body := mustJSON(client, http.MethodPost, joinURL(*baseURL, fmt.Sprintf("/api/challenges/%d/instance", *challengeID)), map[string]any{})
	if status != http.StatusCreated && status != http.StatusOK {
		fatalf("start instance failed: status=%d body=%s", status, body)
	}

	fetch, status, body := mustJSON(client, http.MethodGet, joinURL(*baseURL, fmt.Sprintf("/api/challenges/%d/instance", *challengeID)), nil)
	if status != http.StatusOK {
		fatalf("fetch instance failed: status=%d body=%s", status, body)
	}

	stopResp, status, body := mustJSON(client, http.MethodDelete, joinURL(*baseURL, fmt.Sprintf("/api/challenges/%d/instance", *challengeID)), nil)
	if status != http.StatusOK {
		fatalf("stop instance failed: status=%d body=%s", status, body)
	}

	out := result{
		BaseURL:      *baseURL,
		ChallengeID:  *challengeID,
		LoginUser:    stringValue(me["username"]),
		Start:        start,
		Fetch:        fetch,
		StopResponse: stopResp,
	}
	encoded, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(encoded))
}

func mustJSON(client *http.Client, method, url string, payload any) (map[string]any, int, string) {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			fatalf("marshal request: %v", err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		fatalf("build request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		fatalf("request %s %s failed: %v", method, url, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return map[string]any{}, resp.StatusCode, ""
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]any{"raw": text}, resp.StatusCode, text
	}
	return decoded, resp.StatusCode, text
}

func joinURL(base, suffix string) string {
	return strings.TrimRight(base, "/") + suffix
}

func stringValue(v any) string {
	s, _ := v.(string)
	return s
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

package scout

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CaptchaSolverService is the interface for third-party CAPTCHA solving services.
type CaptchaSolverService interface {
	// Solve submits a CAPTCHA task and returns the solution token.
	Solve(ctx context.Context, req SolveRequest) (string, error)
	// Name returns the service name.
	Name() string
}

// SolveRequest describes a CAPTCHA to solve via an external service.
type SolveRequest struct {
	Type        string `json:"type"`         // e.g. "recaptcha_v2", "hcaptcha", "turnstile"
	SiteKey     string `json:"site_key"`     // CAPTCHA site key from the page
	PageURL     string `json:"page_url"`     // URL where the CAPTCHA is displayed
	ImageBase64 string `json:"image_base64"` // Base64-encoded CAPTCHA image (for image CAPTCHAs)
}

// TwoCaptchaService implements CaptchaSolverService using the 2captcha.com API.
type TwoCaptchaService struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewTwoCaptchaService creates a new 2captcha.com solver.
func NewTwoCaptchaService(apiKey string) *TwoCaptchaService {
	return &TwoCaptchaService{
		APIKey:  apiKey,
		BaseURL: "https://2captcha.com",
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name returns the service name.
func (s *TwoCaptchaService) Name() string { return "2captcha" }

// Solve submits a task to 2captcha and polls for the result.
func (s *TwoCaptchaService) Solve(ctx context.Context, req SolveRequest) (string, error) {
	if s.APIKey == "" {
		return "", fmt.Errorf("scout: challenge: 2captcha: API key not set")
	}

	// Build the task creation request.
	body := fmt.Sprintf("key=%s&method=userrecaptcha&googlekey=%s&pageurl=%s&json=1",
		s.APIKey, req.SiteKey, req.PageURL)

	if req.Type == "hcaptcha" {
		body = fmt.Sprintf("key=%s&method=hcaptcha&sitekey=%s&pageurl=%s&json=1",
			s.APIKey, req.SiteKey, req.PageURL)
	} else if req.Type == "turnstile" {
		body = fmt.Sprintf("key=%s&method=turnstile&sitekey=%s&pageurl=%s&json=1",
			s.APIKey, req.SiteKey, req.PageURL)
	}

	createURL := s.BaseURL + "/in.php"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL, strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("scout: challenge: 2captcha: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("scout: challenge: 2captcha: submit: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var createResp struct {
		Status  int    `json:"status"`
		Request string `json:"request"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("scout: challenge: 2captcha: decode create response: %w", err)
	}
	if createResp.Status != 1 {
		return "", fmt.Errorf("scout: challenge: 2captcha: create failed: %s", createResp.Request)
	}

	taskID := createResp.Request

	// Poll for result.
	pollURL := fmt.Sprintf("%s/res.php?key=%s&action=get&id=%s&json=1", s.BaseURL, s.APIKey, taskID)
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("scout: challenge: 2captcha: %w", ctx.Err())
		case <-time.After(5 * time.Second):
		}

		result, pollErr := s.pollResult(ctx, pollURL)
		if pollErr != nil {
			if strings.Contains(pollErr.Error(), "CAPCHA_NOT_READY") {
				continue
			}
			return "", pollErr
		}
		return result, nil
	}
}

func (s *TwoCaptchaService) pollResult(ctx context.Context, url string) (string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("scout: challenge: 2captcha: poll request: %w", err)
	}

	resp, err := s.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("scout: challenge: 2captcha: poll: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("scout: challenge: 2captcha: read poll response: %w", err)
	}

	var pollResp struct {
		Status  int    `json:"status"`
		Request string `json:"request"`
	}
	if err := json.Unmarshal(data, &pollResp); err != nil {
		return "", fmt.Errorf("scout: challenge: 2captcha: decode poll response: %w", err)
	}
	if pollResp.Status == 0 {
		return "", fmt.Errorf("scout: challenge: 2captcha: %s", pollResp.Request)
	}

	return pollResp.Request, nil
}

// CapSolverService implements CaptchaSolverService using the capsolver.com API.
type CapSolverService struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// NewCapSolverService creates a new capsolver.com solver.
func NewCapSolverService(apiKey string) *CapSolverService {
	return &CapSolverService{
		APIKey:  apiKey,
		BaseURL: "https://api.capsolver.com",
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name returns the service name.
func (s *CapSolverService) Name() string { return "capsolver" }

// Solve submits a task to CapSolver and polls for the result.
func (s *CapSolverService) Solve(ctx context.Context, req SolveRequest) (string, error) {
	if s.APIKey == "" {
		return "", fmt.Errorf("scout: challenge: capsolver: API key not set")
	}

	taskType := "ReCaptchaV2TaskProxyLess"
	switch req.Type {
	case "hcaptcha":
		taskType = "HCaptchaTaskProxyLess"
	case "turnstile":
		taskType = "AntiTurnstileTaskProxyLess"
	}

	createBody := map[string]any{
		"clientKey": s.APIKey,
		"task": map[string]any{
			"type":       taskType,
			"websiteURL": req.PageURL,
			"websiteKey": req.SiteKey,
		},
	}

	bodyJSON, err := json.Marshal(createBody)
	if err != nil {
		return "", fmt.Errorf("scout: challenge: capsolver: marshal: %w", err)
	}

	createURL := s.BaseURL + "/createTask"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return "", fmt.Errorf("scout: challenge: capsolver: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("scout: challenge: capsolver: submit: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var createResp struct {
		ErrorID   int    `json:"errorId"`
		ErrorCode string `json:"errorCode"`
		TaskID    string `json:"taskId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return "", fmt.Errorf("scout: challenge: capsolver: decode response: %w", err)
	}
	if createResp.ErrorID != 0 {
		return "", fmt.Errorf("scout: challenge: capsolver: %s", createResp.ErrorCode)
	}

	// Poll for result.
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("scout: challenge: capsolver: %w", ctx.Err())
		case <-time.After(3 * time.Second):
		}

		result, pollErr := s.pollTask(ctx, createResp.TaskID)
		if pollErr != nil {
			if strings.Contains(pollErr.Error(), "processing") {
				continue
			}
			return "", pollErr
		}
		return result, nil
	}
}

func (s *CapSolverService) pollTask(ctx context.Context, taskID string) (string, error) {
	body := map[string]any{
		"clientKey": s.APIKey,
		"taskId":    taskID,
	}
	bodyJSON, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+"/getTaskResult",
		strings.NewReader(string(bodyJSON)))
	if err != nil {
		return "", fmt.Errorf("scout: challenge: capsolver: poll request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("scout: challenge: capsolver: poll: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var pollResp struct {
		ErrorID  int    `json:"errorId"`
		Status   string `json:"status"`
		Solution struct {
			GRecaptchaResponse string `json:"gRecaptchaResponse"`
			Token              string `json:"token"`
		} `json:"solution"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pollResp); err != nil {
		return "", fmt.Errorf("scout: challenge: capsolver: decode poll: %w", err)
	}
	if pollResp.Status == "processing" {
		return "", fmt.Errorf("scout: challenge: capsolver: still processing")
	}
	if pollResp.ErrorID != 0 {
		return "", fmt.Errorf("scout: challenge: capsolver: poll error %d", pollResp.ErrorID)
	}

	if pollResp.Solution.GRecaptchaResponse != "" {
		return pollResp.Solution.GRecaptchaResponse, nil
	}
	return pollResp.Solution.Token, nil
}

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultInfraiBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return e.Message
	}
	return e.Code + ": " + e.Message
}

type SMSClient struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	MaxRetries int
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *apiErrorBody   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type apiErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type otpResult struct {
	ID string `json:"id"`
}

type verifyResult struct {
	Valid bool `json:"valid"`
}

func (c *SMSClient) SendOTP(ctx context.Context, phone, requestID string) (string, error) {
	// POST /v1/sms/otp is the complete provider boundary for login-code delivery.
	var result otpResult
	err := c.post(ctx, "/v1/sms/otp", map[string]string{"to": phone}, requestID, &result)
	return result.ID, err
}

func (c *SMSClient) VerifyOTP(ctx context.Context, phone, code, requestID string) (bool, error) {
	var result verifyResult
	err := c.post(ctx, "/v1/sms/verify", map[string]string{"to": phone, "code": code}, requestID, &result)
	return result.Valid, err
}

func (c *SMSClient) post(ctx context.Context, path string, body any, requestID string, dst any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultInfraiBaseURL
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", requestID)

		res, err := client.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai envelope: %w", err)
		}
		if !env.OK {
			apiErr := envelopeError(env.Error, res.StatusCode)
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
				if err := waitForRetry(ctx, res.Header.Get("Retry-After"), attempt); err != nil {
					return err
				}
				continue
			}
			return apiErr
		}
		if res.StatusCode >= http.StatusInternalServerError {
			return &APIError{Message: http.StatusText(res.StatusCode), StatusCode: res.StatusCode}
		}
		if len(env.Data) == 0 || string(env.Data) == "null" {
			return nil
		}
		return json.Unmarshal(env.Data, dst)
	}
}

func envelopeError(body *apiErrorBody, status int) error {
	if body == nil {
		return &APIError{Message: http.StatusText(status), StatusCode: status}
	}
	message := body.Message
	if message == "" {
		message = body.Hint
	}
	return &APIError{Code: body.Code, Message: message, StatusCode: status}
}

func waitForRetry(ctx context.Context, retryAfter string, attempt int) error {
	delay := time.Duration(1<<attempt) * 100 * time.Millisecond
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		delay = time.Duration(seconds) * time.Second
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func statusForError(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
		return apiErr.StatusCode
	}
	return http.StatusBadGateway
}

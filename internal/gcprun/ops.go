package gcprun

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var ErrRunOp = apperr.E("C-RUN-003", "Failed to wait for Cloud Run operation")

type operation struct {
	Name string `json:"name"`
	Done bool   `json:"done"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
	Response json.RawMessage `json:"response,omitempty"`
}

func waitOperation(ctx context.Context, accessToken, opName string) error {
	u := fmt.Sprintf("https://run.googleapis.com/v2/%s", opName)

	client := &http.Client{Timeout: 20 * time.Second}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return apperr.New(ErrRunOp).WithCause(ctx.Err()).
				WithMeta("op", opName).
				WithFix("Open Cloud Run console to check deployment status.")
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
			if err != nil {
				return apperr.New(ErrRunOp).WithCause(err)
			}
			req.Header.Set("Authorization", "Bearer "+accessToken)

			res, err := client.Do(req)
			if err != nil {
				return apperr.New(ErrRunOp).WithCause(err)
			}
			raw, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()

			if res.StatusCode < 200 || res.StatusCode >= 300 {
				return apperr.New(ErrRunOp).
					WithMeta("http_status", res.Status).
					WithMeta("raw_body", string(raw)).
					WithMeta("op", opName)
			}

			var op operation
			if err := json.Unmarshal(raw, &op); err != nil {
				return apperr.New(ErrRunOp).WithCause(err).
					WithMeta("raw_body", string(raw))
			}

			if !op.Done {
				continue
			}
			if op.Error != nil {
				return apperr.New(ErrRunOp).
							WithMeta("op", opName).
							WithMeta("error_code", fmt.Sprintf("%d", op.Error.Code)).
							WithMeta("error_status", op.Error.Status).
							WithMeta("error_message", op.Error.Message)
			}
			return nil
		}
	}
}
package client

import (
	"context"
	"fmt"
	"net/http"
)

type TableSummary struct {
	TableName string `json:"table_name"`
	TableGUID string `json:"table_guid"`
}

type Table struct {
	Summary TableSummary
	ss      *Session
}

// Start() starts an existing table, so the game routine can start.
func (t *Table) Start(ctx context.Context) error {
	s := t.ss
	url := s.url.JoinPath("tables", t.Summary.TableGUID, "start")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.cli.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("wrong return code %v", resp.StatusCode)
	}
	return nil
}

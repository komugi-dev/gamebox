package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type Session struct {
	cli *http.Client
	url url.URL
}

// CreateSession() creates a new game session.
// A session allows to create a new table get a list of existing tables.
// Create a session first, then a table and a player.
func CreateSession(cli *http.Client, gameboxURL url.URL) *Session {
	if cli == nil {
		cli = &http.Client{Timeout: ClientTimeout}
	}
	return &Session{
		cli: cli,
		url: gameboxURL,
	}
}

// Table() creates a table with the given name.
// It returns the table id on success.
func (s *Session) Table(ctx context.Context, name string) (Table, error) {

	url := s.url.JoinPath("tables")
	ts := TableSummary{
		TableName: name,
	}
	buf, err := json.Marshal(ts)
	if err != nil {
		return Table{}, fmt.Errorf("failed to encode table [%v][%v]", ts, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url.String(), bytes.NewReader(buf))
	if err != nil {
		return Table{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.cli.Do(req)
	if err != nil {
		return Table{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		return Table{}, fmt.Errorf("wrong return code %v", resp.StatusCode)
	}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&ts)
	if err != nil {
		return Table{}, err
	}

	return Table{Summary: ts, ss: s}, nil
}

// Tables() returns a list of tables.
func (s *Session) Tables(ctx context.Context) ([]Table, error) {
	url := s.url.JoinPath("tables")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return []Table{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.cli.Do(req)
	if err != nil {
		return []Table{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return []Table{}, fmt.Errorf("wrong return code %v", resp.StatusCode)
	}
	ts := []TableSummary{}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&ts)
	if err != nil {
		return []Table{}, err
	}

	tables := []Table{}
	for _, t := range ts {
		tables = append(tables, Table{
			Summary: t,
			ss:      s,
		})
	}

	return tables, nil
}

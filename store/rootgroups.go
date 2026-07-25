package store

import (
	"context"
	"fmt"
	"strings"
)

// RootGroupMember is one root inside a priority group.
type RootGroupMember struct {
	RootName string `json:"rootName"`
	Priority int    `json:"priority"`
}

// UpsertRootGroup replaces membership for a named root group.
func (s *Store) UpsertRootGroup(ctx context.Context, name, description string, members []RootGroupMember) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("group name is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO root_groups(name, description) VALUES(?, ?)
		ON CONFLICT(name) DO UPDATE SET description=excluded.description`, name, description); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM root_group_members WHERE group_name=?`, name); err != nil {
		return err
	}
	for _, m := range members {
		if strings.TrimSpace(m.RootName) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO root_group_members(group_name, root_name, priority) VALUES(?,?,?)`,
			name, m.RootName, m.Priority); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListRootGroupMembers returns roots ordered by priority desc.
func (s *Store) ListRootGroupMembers(ctx context.Context, group string) ([]RootGroupMember, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT root_name, priority FROM root_group_members
		WHERE group_name=? ORDER BY priority DESC, root_name`, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RootGroupMember
	for rows.Next() {
		var m RootGroupMember
		if err := rows.Scan(&m.RootName, &m.Priority); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ExpandAliases returns canonical forms for a query token (bounded).
func (s *Store) ExpandAliases(ctx context.Context, token, technology string, limit int) ([]string, error) {
	token = strings.TrimSpace(strings.ToLower(token))
	if token == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	q := `SELECT DISTINCT canonical FROM aliases WHERE lower(alias)=?`
	args := []any{token}
	if technology != "" {
		q += ` AND (technology='' OR technology=?)`
		args = append(args, technology)
	}
	q += ` LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpsertAlias stores a controlled alias mapping.
func (s *Store) UpsertAlias(ctx context.Context, canonical, alias, technology, rootName string, confidence float64) error {
	if strings.TrimSpace(canonical) == "" || strings.TrimSpace(alias) == "" {
		return fmt.Errorf("canonical and alias are required")
	}
	if confidence <= 0 {
		confidence = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO aliases(canonical, alias, technology, root_name, confidence)
		VALUES(?,?,?,?,?)
		ON CONFLICT(alias, technology, root_name) DO UPDATE SET
			canonical=excluded.canonical, confidence=excluded.confidence`,
		canonical, alias, technology, rootName, confidence)
	return err
}

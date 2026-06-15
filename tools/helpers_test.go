package tools

import (
	"encoding/json"
	"testing"

	"github.com/cockroachdb/cockroachdb-mcp-server/db"
	"github.com/stretchr/testify/require"
)

func TestApplyLimitOffset(t *testing.T) {
	mk := func(v int64) *int64 { return &v }
	h := newHandlers(nil)

	t.Run("uses default when limit is nil", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", nil, nil)
		require.NoError(t, err)
		require.Equal(t, "SELECT 1 LIMIT 100", got)
	})

	t.Run("respects explicit limit", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", mk(7), nil)
		require.NoError(t, err)
		require.Equal(t, "SELECT 1 LIMIT 7", got)
	})

	t.Run("appends offset", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", mk(7), mk(3))
		require.NoError(t, err)
		require.Equal(t, "SELECT 1 LIMIT 7 OFFSET 3", got)
	})

	t.Run("caps limit at maximum", func(t *testing.T) {
		got, err := h.applyLimitOffset("SELECT 1", mk(999_999), nil)
		require.NoError(t, err)
		require.Contains(t, got, "LIMIT 10000")
	})

	t.Run("rejects non-positive limit and negative offset", func(t *testing.T) {
		_, err := h.applyLimitOffset("SELECT 1", mk(0), nil)
		require.Error(t, err, "expected error for zero limit")
		_, err = h.applyLimitOffset("SELECT 1", mk(-1), nil)
		require.Error(t, err, "expected error for negative limit")
		_, err = h.applyLimitOffset("SELECT 1", nil, mk(-1))
		require.Error(t, err, "expected error for negative offset")
	})
}

func TestQueryResultToMCP(t *testing.T) {
	t.Run("renders rows as objects keyed by column", func(t *testing.T) {
		res, err := queryResultToMCP(&db.QueryResult{
			Columns: []string{"id", "name"},
			Rows: [][]any{
				{int64(1), "alice"},
				{int64(2), "bob"},
			},
		})
		require.NoError(t, err)
		text := textOf(t, res)
		var payload struct {
			Rows []map[string]any `json:"rows"`
		}
		require.NoError(t, json.Unmarshal([]byte(text), &payload), "payload=%s", text)
		require.Len(t, payload.Rows, 2)
		require.Equal(t, "alice", payload.Rows[0]["name"])
	})

	t.Run("emits empty rows array when result is empty", func(t *testing.T) {
		res, err := queryResultToMCP(&db.QueryResult{Columns: []string{"x"}})
		require.NoError(t, err)
		require.Contains(t, textOf(t, res), `"rows":[]`)
	})
}

// TestEnsureSelectOnly is the security primitive for select_query and
// explain_query. Every smuggling shape the validator is supposed to reject
// gets a case here.
func TestEnsureSelectOnly(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		// Happy paths.
		{"plain SELECT", "SELECT 1", false},
		{"CTE with SELECT", "WITH x AS (SELECT 1) SELECT * FROM x", false},
		{"UNION of SELECTs", "SELECT 1 UNION SELECT 2", false},
		{"bracket subquery with SELECT", "SELECT * FROM [SELECT 1]", false},
		{"expression subquery with SELECT", "SELECT (SELECT 1)", false},
		{"nested CTE all SELECTs", "WITH a AS (WITH b AS (SELECT 1) SELECT * FROM b) SELECT * FROM a", false},

		// Top-level DML / DDL.
		{"top-level DELETE", "DELETE FROM t", true},
		{"top-level INSERT", "INSERT INTO t VALUES (1)", true},
		{"top-level UPDATE", "UPDATE t SET a = 1", true},
		{"top-level UPSERT", "UPSERT INTO t VALUES (1)", true},
		{"top-level CREATE TABLE", "CREATE TABLE t (a INT)", true},
		{"top-level DROP TABLE", "DROP TABLE t", true},

		// DML in CTE - headline claim from the PR description.
		{"CTE with DELETE", "WITH x AS (DELETE FROM t RETURNING *) SELECT * FROM x", true},
		{"CTE with INSERT", "WITH x AS (INSERT INTO t VALUES (1) RETURNING *) SELECT * FROM x", true},
		{"CTE with UPDATE", "WITH x AS (UPDATE t SET a = 1 RETURNING *) SELECT * FROM x", true},

		// DML in bracket subquery (StatementSource) - headline claim.
		{"bracket subquery with DELETE", "SELECT * FROM [DELETE FROM t RETURNING *]", true},
		{"bracket subquery with INSERT", "SELECT * FROM [INSERT INTO t VALUES (1) RETURNING *]", true},
		{"bracket subquery with UPDATE", "SELECT * FROM [UPDATE t SET a = 1 RETURNING *]", true},

		// Nested DML smuggling.
		{"nested CTE with DML inside",
			"WITH a AS (WITH b AS (DELETE FROM t RETURNING *) SELECT * FROM b) SELECT * FROM a", true},
		{"UNION with bracket-subquery DML",
			"SELECT 1 UNION SELECT id FROM [DELETE FROM t RETURNING id]", true},
		{"bracket subquery with CTE DML",
			"SELECT * FROM [WITH x AS (DELETE FROM t RETURNING *) SELECT * FROM x]", true},
		{"expression subquery with CTE DML",
			"SELECT (WITH x AS (DELETE FROM t RETURNING id) SELECT id FROM x LIMIT 1)", true},
		{"FROM-clause parenthesised SELECT with CTE DML",
			"SELECT * FROM (WITH x AS (DELETE FROM t RETURNING *) SELECT * FROM x) AS s", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmt, err := parseSingleStatement(tc.sql)
			require.NoError(t, err, "parse")
			_, err = ensureSelectOnly(stmt)
			if tc.wantErr {
				require.Errorf(t, err, "validator should reject %q", tc.sql)
			} else {
				require.NoErrorf(t, err, "validator should allow %q", tc.sql)
			}
		})
	}
}

// TestValidateShowStatement enforces the thin guardrail: any SHOW is allowed
// (CRDB's privilege model decides what the connected role can actually run);
// non-SHOW statements are rejected.
func TestValidateShowStatement(t *testing.T) {
	cases := []struct {
		name    string
		sql     string
		wantErr bool
	}{
		// Schema / topology SHOWs.
		{"SHOW DATABASES", "SHOW DATABASES", false},
		{"SHOW SCHEMAS", "SHOW SCHEMAS", false},
		{"SHOW TABLES", "SHOW TABLES", false},
		{"SHOW COLUMNS", "SHOW COLUMNS FROM t", false},
		{"SHOW INDEXES", "SHOW INDEXES FROM t", false},
		{"SHOW CREATE", "SHOW CREATE TABLE t", false},
		{"SHOW REGIONS", "SHOW REGIONS", false},
		{"SHOW ZONE CONFIG", "SHOW ZONE CONFIGURATION FROM TABLE t", false},

		// Operational SHOWs - allowed; CRDB enforces privileges at execution.
		{"SHOW JOBS", "SHOW JOBS", false},
		{"SHOW QUERIES", "SHOW QUERIES", false},
		{"SHOW SESSIONS", "SHOW SESSIONS", false},
		{"SHOW STATISTICS", "SHOW STATISTICS FOR TABLE t", false},

		// Non-SHOW statements rejected.
		{"SELECT rejected", "SELECT 1", true},
		{"DELETE rejected", "DELETE FROM t", true},
		{"CREATE TABLE rejected", "CREATE TABLE t (a INT)", true},
		{"SET rejected", "SET application_name = 'x'", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stmt, err := parseSingleStatement(tc.sql)
			require.NoError(t, err, "parse")
			err = validateShowStatement(stmt)
			if tc.wantErr {
				require.Errorf(t, err, "validator should reject %q", tc.sql)
			} else {
				require.NoErrorf(t, err, "validator should allow %q", tc.sql)
			}
		})
	}
}

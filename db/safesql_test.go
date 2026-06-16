package db

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSafeFormat(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cases := []struct {
			name, format, want string
			args               []any
		}{
			{"quotes identifier and literal",
				"SELECT * FROM %1 WHERE name = %2",
				`SELECT * FROM "orders" WHERE name = e'ali\'ce'`,
				[]any{Identifier("orders"), "ali'ce"}},
			{"quotes literal with no escapes plainly",
				"WHERE name = %1", `WHERE name = 'alice'`, []any{"alice"}},
			{"supports qualified identifiers",
				"SHOW CREATE TABLE %1", `SHOW CREATE TABLE "db"."schema"."tbl"`,
				[]any{QualifiedIdentifier{"db", "schema", "tbl"}}},
			{"formats int64 arguments",
				"LIMIT %1 OFFSET %2", "LIMIT 50 OFFSET 10",
				[]any{int64(50), int64(10)}},
			{"inlines raw SQL",
				"%1 LIMIT %2", "SELECT 1 LIMIT 10",
				[]any{SQL("SELECT 1"), int64(10)}},
			{"allows placeholder after punctuation",
				"(%1,%2)", "(1,2)", []any{int64(1), int64(2)}},
			{"allows placeholder after equals",
				"WHERE id=%1", "WHERE id=42", []any{int64(42)}},
			{"allows placeholder at format start",
				"%1 LIMIT %2", "SELECT 1 LIMIT 10",
				[]any{SQL("SELECT 1"), int64(10)}},
			{"passes through format with no placeholders",
				"SELECT 1", "SELECT 1", nil},
			{"allows same arg referenced multiple times",
				"WHERE a = %1 OR b = %1", "WHERE a = 7 OR b = 7",
				[]any{int64(7)}},
			{"supports multi-digit placeholder index",
				"%10",
				`"t10"`,
				[]any{
					Identifier("t1"), Identifier("t2"), Identifier("t3"),
					Identifier("t4"), Identifier("t5"), Identifier("t6"),
					Identifier("t7"), Identifier("t8"), Identifier("t9"),
					Identifier("t10"),
				}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := SafeFormat(tc.format, tc.args...)
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			})
		}
	})

	t.Run("error", func(t *testing.T) {
		cases := []struct {
			name, format, contains string
			args                   []any
		}{
			{"out-of-range placeholder", "SELECT %1, %2", "out of range",
				[]any{"only-one"}},
			{"unsupported type", "%1", "unsupported type", []any{3.14}},
			{"placeholder adjacent to letter", "col%1", "punctuation or whitespace",
				[]any{Identifier("x")}},
			{"malformed placeholder", "SELECT %a", "invalid placeholder",
				[]any{Identifier("x")}},
			{"placeholder index too large", "SELECT %1000000000", "too large",
				[]any{Identifier("x")}},
			{"trailing percent with no digit", "SELECT %", "invalid placeholder",
				[]any{int64(1)}},
			{"zero-index placeholder", "SELECT %0", "invalid placeholder",
				[]any{int64(1)}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := SafeFormat(tc.format, tc.args...)
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.contains)
			})
		}
	})
}

func TestIdentifierEscaping(t *testing.T) {
	cases := []struct {
		name, got, want string
	}{
		{"escapes embedded quotes",
			Identifier(`weird"name`).SQLString(), `"weird""name"`},
		{"always wraps in double quotes",
			Identifier("plain").SQLString(), `"plain"`},
		{"qualified identifier joins parts with dots",
			QualifiedIdentifier{"db", "public", "tbl"}.SQLString(), `"db"."public"."tbl"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.got)
		})
	}
}

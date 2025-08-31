package utils

import (
    "reflect"
    "testing"
)

func TestSplitStatements(t *testing.T) {
    t.Run("basic", func(t *testing.T) {
        in := "CREATE TABLE t (id INT); INSERT INTO t VALUES (1);"
        want := []string{
            "CREATE TABLE t (id INT)",
            "INSERT INTO t VALUES (1)",
        }
        got := SplitStatements(in)
        if !reflect.DeepEqual(got, want) {
            t.Fatalf("got %+v, want %+v", got, want)
        }
    })

    t.Run("trailing and no terminator", func(t *testing.T) {
        in := "SELECT 1; SELECT 2"
        want := []string{"SELECT 1", "SELECT 2"}
        got := SplitStatements(in)
        if !reflect.DeepEqual(got, want) {
            t.Fatalf("got %+v, want %+v", got, want)
        }
    })

    t.Run("quotes", func(t *testing.T) {
        cases := []struct {
            name string
            in   string
            want []string
        }{
            {
                name: "single quotes",
                in:   "INSERT INTO t (s) VALUES ('a;b'); SELECT 2;",
                want: []string{
                    "INSERT INTO t (s) VALUES ('a;b')",
                    "SELECT 2",
                },
            },
            {
                name: "double quotes",
                in:   "CREATE TABLE \"a;b\" (id INT);",
                want: []string{"CREATE TABLE \"a;b\" (id INT)"},
            },
            {
                name: "backticks",
                in:   "CREATE TABLE `a;b` (id INT); SELECT 1;",
                want: []string{
                    "CREATE TABLE `a;b` (id INT)",
                    "SELECT 1",
                },
            },
            {
                name: "escaped single quote with backslash",
                in:   "INSERT INTO t (s) VALUES ('a\\'b; c'); SELECT 1;",
                want: []string{
                    "INSERT INTO t (s) VALUES ('a\\'b; c')",
                    "SELECT 1",
                },
            },
            {
                name: "doubled single quote",
                in:   "INSERT INTO t (s) VALUES ('a''b; c'); SELECT 1;",
                want: []string{
                    "INSERT INTO t (s) VALUES ('a''b; c')",
                    "SELECT 1",
                },
            },
        }
        for _, tc := range cases {
            t.Run(tc.name, func(t *testing.T) {
                got := SplitStatements(tc.in)
                if !reflect.DeepEqual(got, tc.want) {
                    t.Fatalf("%s: got %+v, want %+v", tc.name, got, tc.want)
                }
            })
        }
    })

    t.Run("comments", func(t *testing.T) {
        in := `-- line comment; should be ignored
SELECT 1; /* block; comment */ SELECT 2; /* nested /* inner */ still */ SELECT 3;`
        want := []string{"SELECT 1", "SELECT 2", "SELECT 3"}
        got := SplitStatements(in)
        if !reflect.DeepEqual(got, want) {
            t.Fatalf("got %+v, want %+v", got, want)
        }
    })

    t.Run("dollar quotes", func(t *testing.T) {
        cases := []struct {
            name string
            in   string
            want []string
        }{
            {
                name: "unnamed $$",
                in:   "DO $$ BEGIN RAISE NOTICE 'x;'; END $$; SELECT 1;",
                want: []string{
                    "DO $$ BEGIN RAISE NOTICE 'x;'; END $$",
                    "SELECT 1",
                },
            },
            {
                name: "tagged $q$",
                in:   "DO $q$ BEGIN PERFORM 1; -- not a split\n END $q$; SELECT 2;",
                want: []string{
                    "DO $q$ BEGIN PERFORM 1; -- not a split\n END $q$",
                    "SELECT 2",
                },
            },
        }
        for _, tc := range cases {
            t.Run(tc.name, func(t *testing.T) {
                got := SplitStatements(tc.in)
                if !reflect.DeepEqual(got, tc.want) {
                    t.Fatalf("%s: got %#v, want %#v", tc.name, got, tc.want)
                }
            })
        }
    })

    t.Run("empty and whitespace", func(t *testing.T) {
        in := " ;\n; SELECT 1;; ; SELECT 2; ;"
        want := []string{"SELECT 1", "SELECT 2"}
        got := SplitStatements(in)
        if !reflect.DeepEqual(got, want) {
            t.Fatalf("got %+v, want %+v", got, want)
        }
    })
}

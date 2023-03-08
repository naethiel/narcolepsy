package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var envT = &Environment{
	"foo": "bar",
}

func readLinesT(t *testing.T) []string {
	t.Helper()

	lines, err := readLines("./testdata/" + t.Name() + ".http")
	if err != nil {
		t.Fatal(err)
	}

	return lines
}

func Test_GetRawRequests(t *testing.T) {
	t.Run("nominal", func(t *testing.T) {
		lines := readLinesT(t)
		out := getReqDumps(envT, lines)

		expected := []RequestDump{
			{
				Key: "Create user",
				Value: `POST http://localhost:3000/user HTTP/1.1
Content-Type:application/json
Fake-Header: Tamer
Foo:bar

{
	"foo": "bar"
}
`,
			},
		}

		assert.Equal(t, expected, out)
	})

	t.Run("multiple requests", func(t *testing.T) {
		lines := readLinesT(t)
		out := getReqDumps(envT, lines)

		expected := []RequestDump{
			{
				Key: "First",
				Value: `POST http://localhost:3000/user HTTP/1.1
Content-Type:application/json
Fake-Header: Tamer
Foo:bar

{
	"foo": "bar"
}
`},
			{
				Key: "Second",
				Value: `PUT http://localhost:3000/user HTTP/1.1
Content-Type:application/json
Fake-Header: Tamer
Foo:bar

{
	"foo": "bar"
}
`},
			{
				Key: "Third",
				Value: `GET http://localhost:3000/user HTTP/1.1
Content-Type:application/json
`},
		}

		assert.Equal(t, expected, out)
	})
	t.Run("random line breaks", func(t *testing.T) {
		lines := readLinesT(t)
		out := getReqDumps(envT, lines)

		expected := []RequestDump{
			{
				Key: "Separator",
				Value: `GET http://google.com HTTP/1.1
Fake-header:foo
baz:bar
`,
			},
		}

		assert.Equal(t, expected, out)

	})
	t.Run("missing method or protocol", func(t *testing.T) {
		lines := readLinesT(t)
		out := getReqDumps(envT, lines)

		expected := []RequestDump{
			{
				Key: "missing method",
				Value: `GET http://google.com HTTP/1.1
Fake-header:foo
baz:bar
`,
			},
			{
				Key: "missing proto",
				Value: `PUT http://google.com HTTP/1.1
Fake-header:foo
baz:bar
`,
			},
		}

		assert.Equal(t, expected, out)
	})
}

func TestApplyEnvVars(t *testing.T) {
	env := &Environment{
		"darth-vader": "anakin-skywalker",
		"C3PO":        "R2D2",
	}
	t.Run("nominal", func(t *testing.T) {
		applied := applyEnvVars(env, "{darth-vader}: jedi")
		assert.Equal(t, "anakin-skywalker: jedi", applied)
	})
	t.Run("replace 2 things", func(t *testing.T) {
		applied := applyEnvVars(env, "{darth-vader}:{C3PO}")
		assert.Equal(t, "anakin-skywalker:R2D2", applied)
	})
	t.Run("unknown vars", func(t *testing.T) {
		applied := applyEnvVars(env, "{foobar}: {2000}")
		assert.Equal(t, "{foobar}: {2000}", applied)
	})
	t.Run("bad syntax", func(t *testing.T) {
		applied := applyEnvVars(env, "{darth-vader:{C3PO")
		assert.Equal(t, "{darth-vader:{C3PO", applied)
	})
}

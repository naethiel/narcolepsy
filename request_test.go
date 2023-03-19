package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var envT = Environment{
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

	t.Run("multiple requests", func(t *testing.T) {
		lines := readLinesT(t)
		out := getRawRequests(envT, lines)

		expected := []string{
			`### First

POST http://localhost:3000/user HTTP/1.1
Content-Type:application/json
Fake-Header: Tamer
Foo:bar

{
	"foo": "bar"
}`,
			`### Second

PUT http://localhost:3000/user HTTP/1.1
Content-Type:application/json
Fake-Header: Tamer
Foo:bar

{
	"foo": "bar"
}`,
			`### Third

GET http://localhost:3000/user HTTP/1.1
Content-Type:application/json`,
		}

		assert.Equal(t, expected, out)
	})
	t.Run("no initial separator", func(t *testing.T) {
		lines := readLinesT(t)
		out := getRawRequests(envT, lines)

		expected := []string{
			// use first request line as key
			`POST http://localhost:3000/user HTTP/1.1
Content-Type:application/json
Fake-Header: Tamer
Foo:bar

{
	"foo": "bar"
}`,
			`### Second

PUT http://localhost:3000/user HTTP/1.1
Content-Type:application/json
Fake-Header: Tamer
Foo:bar

{
	"foo": "bar"
}`,
			`### Third

GET http://localhost:3000/user HTTP/1.1
Content-Type:application/json`,
		}

		assert.Equal(t, expected, out)
	})
}

func TestApplyEnvVars(t *testing.T) {
	env := Environment{
		"darth-vader": "anakin-skywalker",
		"C3PO":        "R2D2",
	}
	t.Run("nominal", func(t *testing.T) {
		applied := applyEnvVars(env, "{{darth-vader}}: jedi")
		assert.Equal(t, "anakin-skywalker: jedi", applied)
	})
	t.Run("replace 2 things", func(t *testing.T) {
		applied := applyEnvVars(env, "{{darth-vader}}:{{C3PO}}")
		assert.Equal(t, "anakin-skywalker:R2D2", applied)
	})
	t.Run("unknown vars", func(t *testing.T) {
		applied := applyEnvVars(env, "{{foobar}}: {{2000}}")
		assert.Equal(t, "{{foobar}}: {{2000}}", applied)
	})
	t.Run("bad syntax", func(t *testing.T) {
		applied := applyEnvVars(env, "{{darth-vader:{{C3PO")
		assert.Equal(t, "{{darth-vader:{{C3PO", applied)
	})
}

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
		out := getRawRequests(lines)

		expected := []rawRequest{
			{
				Key: "Create user",
				Definition: `POST http://localhost:3000/user HTTP/1.1
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
		out := getRawRequests(lines)

		expected := []rawRequest{
			{
				Key: "First",
				Definition: `POST http://localhost:3000/user HTTP/1.1
Content-Type:application/json
Fake-Header: Tamer
Foo:bar

{
	"foo": "bar"
}
`},
			{
				Key: "Second",
				Definition: `PUT http://localhost:3000/user HTTP/1.1
Content-Type:application/json
Fake-Header: Tamer
Foo:bar

{
	"foo": "bar"
}
`},
			{
				Key: "Third",
				Definition: `GET http://localhost:3000/user HTTP/1.1
Content-Type:application/json
`},
		}

		assert.Equal(t, expected, out)
	})
	t.Run("random line breaks", func(t *testing.T) {
		lines := readLinesT(t)
		out := getRawRequests(lines)

		expected := []rawRequest{
			{
				Key: "Separator",
				Definition: `GET http://google.com HTTP/1.1
Fake-header:foo
baz:bar
`,
			},
		}

		assert.Equal(t, expected, out)

	})
}

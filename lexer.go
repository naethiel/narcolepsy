package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/inconshreveable/log15"
	"golang.org/x/exp/slices"
)

const (
	COMMENT_TOKEN    = "COMMENT"
	SEPARATOR_TOKEN  = "SEPARATOR"
	METHOD_TOKEN     = "METHOD"
	TARGET_TOKEN     = "TARGET"
	PROTOCOL_TOKEN   = "PROTOCOL"
	HEADER_FIELD     = "HEADER_FIELD"
	HEADER_SEPARATOR = "HEADER_SEPARATOR"
	HEADER_VALUE     = "HEADER_VALUE"
	REQUEST_BODY     = "REQUEST_BODY"
)

var allowedMethods = []string{
	"GET",
	"HEAD",
	"POST",
	"PUT",
	"DELETE",
	"CONNECT",
	"PATCH",
	"OPTIONS",
	"TRACE",
}

type Token struct {
	Val string
	Typ string
}

type lexer struct {
	input  string
	start  int // start position of current token in input
	head   int // current HEAD position in input
	width  int // size of the last rune read
	err    error
	tokens []Token
}

func (l *lexer) next() rune {
	// avoid out-of-bonds access
	if l.head >= len(l.input) {
		l.err = io.EOF
		return 0
	}

	rune, width := utf8.DecodeRuneInString(l.input[l.head:])
	l.head += width
	l.width = width

	return rune
}

func (l *lexer) readLine() string {
	line := strings.Builder{}
	for {
		r := l.next()

		if l.err != nil {
			break
		}

		if r == '\n' {
			l.back()
			break
		}
		line.WriteRune(r)
	}

	return line.String()
}

func (l *lexer) ignore() {
	l.start = l.head
}

func (l *lexer) back() {
	l.head -= l.width
}

func (l *lexer) peek() rune {
	r := l.next()
	l.back()

	return r
}

func (l *lexer) accept(valid string) bool {
	r := l.next()
	if strings.ContainsRune(valid, r) {
		return true
	}
	l.back()
	return false
}

func (l *lexer) acceptMatch(predicate func(rune) bool) bool {
	r := l.next()
	if predicate(r) {
		return true
	}
	l.back()
	return false
}

func (l *lexer) acceptMatchRun(predicate func(rune) bool) {
	for {
		r := l.next()
		if !predicate(r) {
			break
		}
	}
	l.back()
}

func (l *lexer) acceptRun(valid string) {
	for {
		r := l.next()
		if !strings.ContainsRune(valid, r) {
			break
		}
	}

	l.back()
}

func (l *lexer) acceptString(valid string) bool {
	prefix := l.peekN(len(valid))

	if prefix == valid {
		l.head += len(valid)
		return true
	}
	return false
}

func (l *lexer) peekN(n int) string {
	if l.head+n >= len(l.input) {
		n = len(l.input) - l.head
	}

	return l.input[l.head : l.head+n]
}

func (l *lexer) getSelection() string {
	return l.input[l.start:l.head]
}

func (l *lexer) emit(typ string) {
	s := l.getSelection()
	log15.Debug("emitting token", "token", s, "type", typ, "len", len(s))
	l.start = l.head
	l.tokens = append(l.tokens, Token{Val: s, Typ: typ})
}

type StateFn func(*lexer) StateFn

func lex(s string) ([]Token, error) {
	l := &lexer{
		input: s,
		head:  0,
		start: 0,
	}

	state := lexStart
	for state != nil && l.err == nil {
		state = state(l)
	}

	if l.err == io.EOF {
		l.err = nil
	}
	if l.err != nil {
		return nil, fmt.Errorf("lexing error: %w", l.err)
	}

	return l.tokens, nil
}

func lexStart(l *lexer) StateFn {
	for {
		r := l.next()
		if l.err != nil {
			break
		}

		switch {
		// comments start with either // or #
		case r == '/':
			next := l.peek()

			if next == r {
				return lexComment
			}
		case r == '#':
			l.back()
			return lexComment
		// letter means we may have content here, starting with
		// a request definition line
		case unicode.IsLetter(r):
			return lexRequestLine
		default:
			l.ignore()
		}

	}
	return nil
}

func lexRequestLine(l *lexer) StateFn {
	// peek next word
	nextWords := strings.Fields(l.input[l.start:])

	if len(nextWords) == 0 {
		l.err = errors.New("Lexing request line: only whitespace found")
	}

	// if next word is method, then lex method
	if slices.Contains(allowedMethods, nextWords[0]) {
		return lexMethod
	}

	// else we may be having a request target, lex that
	return lexRequestTarget
}

func lexProtocol(l *lexer) StateFn {
	// skipping the "HTTP/" part
	l.acceptString("HTTP/")

	l.acceptMatch(unicode.IsDigit)

	if l.accept(".") {
		l.acceptMatch(unicode.IsDigit)
	}

	l.emit(PROTOCOL_TOKEN)

	ok := l.accept("\n")
	if !ok {
		return nil
	}

	l.ignore()
	// protocol is last thing in request line
	// next are headers or request body
	return lexAfterRequestLine
}

func lexMethod(l *lexer) StateFn {
	l.acceptMatchRun(unicode.IsUpper)
	// after method we have 1 whitespace

	l.emit(METHOD_TOKEN)

	ok := l.accept(" ")
	if !ok {
		l.err = fmt.Errorf("lexing Method: expected whitespace after method, got %q", l.peek())
		return nil
	}
	l.ignore()

	// next after a method is a request target uri
	return lexRequestTarget
}

func lexRequestTarget(l *lexer) StateFn {
	// move forward as long as we don't meet a whitespace or linebreak
	for {
		r := l.next()

		if l.err != nil {
			break
		}

		if unicode.IsSpace(r) {
			l.back()
			break
		}
	}

	// try to parse what we got as URL
	_, err := url.Parse(l.getSelection())
	if err != nil {
		l.err = fmt.Errorf("lexing request target : %w", err)
		return nil
	}

	// if valid, emit token
	l.emit(TARGET_TOKEN)

	// check what comes next
	if l.accept("\n") {
		// got EOL, protocol is thus missing, should now parse headers
		l.ignore()
		return lexAfterRequestLine
	}

	if l.accept(" ") {
		// got whitespace, skip it and try to parse protocol
		l.ignore()
		return lexProtocol
	}

	// file may validly EOF after request lexRequestTarget
	if l.err == io.EOF {
		return nil
	}

	// unexpected char after target, fail
	l.err = fmt.Errorf("unexpected char after request target: got %q, expected whitespace or \n. Err: %w", l.peek(), l.err)
	return nil
}

func lexComment(l *lexer) StateFn {

	// comments always run to end of line
	line := l.readLine()

	// check if this is a separator or just a normal comment
	if strings.HasPrefix(line, "###") {
		// we have "###" so this is a separator
		l.emit(SEPARATOR_TOKEN)
	} else {
		l.emit(COMMENT_TOKEN)
	}

	// ignore line break after comment
	l.accept("\n")
	l.ignore()
	// after parsing a comment
	// return to default lexer
	return lexStart
}

func lexAfterRequestLine(l *lexer) StateFn {
	if l.accept("/#") {
		// line is comment, so switch to comment mode
		log15.Debug("now should lex comment")
		return lexComment
	}
	if l.accept("\n") {
		// we have an empty line, next comes the body
		log15.Debug("blank line after request line, lex body now")
		return lexRequestBody
	}

	// try reading headers
	return lexHeaderLine
}

func lexHeaderLine(l *lexer) StateFn {
	// accept any char except ":"
	l.acceptMatchRun(func(r rune) bool {
		return r != '\n' && r != ':'
	})

	l.emit(HEADER_FIELD)

	// require a ":" char after header field
	if !l.accept(":") {
		l.err = fmt.Errorf("lexing header line: expected \":\" separator, got %q", l.peek())
		return nil
	}

	l.emit(HEADER_SEPARATOR)

	// accept one optional whitespace
	if l.accept(" ") {
		l.ignore()
	}

	// walk through to the end of the line
	l.readLine()
	l.emit(HEADER_VALUE)

	// skip line break and start over
	l.accept("\n")
	l.ignore()

	return lexAfterRequestLine
}

func lexRequestBody(l *lexer) StateFn {
	for {
		l.readLine()
		if l.accept("\n") {
			next := l.peekN(3)
			if isComment(next) {
				break
			}

		}
	}

	l.emit(REQUEST_BODY)
	return lexStart
}

func isComment(s string) bool {
	return strings.HasPrefix(s, "###") || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "//")
}

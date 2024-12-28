// Copyright 2017, 2024 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/UNO-SOFT/zlog/v2"
	oracall "github.com/tgulacsi/oracall/lib"
)

var rDecl = regexp.MustCompile(`(FUNCTION|PROCEDURE) +([^ (;]+)`)

func ParseDocs(ctx context.Context, src string) (docs map[string]string, err error) {
	logger := zlog.SFromContext(ctx)
	docs, err = parseDocs(ctx, src)
	if err != nil {
		logger.Error("parseDocs", "docs", len(docs), "error", err)
	}
	return docs, err
}

func Parse(ctx context.Context, src string) (docs map[string]string, annotations []oracall.Annotation, err error) {
	logger := zlog.SFromContext(ctx)
	if annotations, err = parseAnnotations(ctx, src); err != nil {
		return nil, annotations, err
	}
	if len(annotations) != 0 {
		logger.Debug("found", "annotations", annotations)
		src = rAnnotation.ReplaceAllString(src, "")
	}
	docs, err = parseDocs(ctx, src)
	if err != nil {
		logger.Error("parseDocs", "docs", len(docs), "error", err)
	}
	return docs, annotations, err
}

var rAnnotation = regexp.MustCompile(`--oracall:(?:(replace(_json)?|rename|tag)\s+[a-zA-Z0-9_#]+\s*=>\s*.+|(handle|private)\s+[a-zA-Z0-9_#]+|max-table-size\s+[a-zA-Z0-9_$]+\s*=\s*[0-9]+)`)

func parseAnnotations(ctx context.Context, src string) ([]oracall.Annotation, error) {
	// logger := zlog.SFromContext(ctx)
	var annotations []oracall.Annotation
	for _, b := range rAnnotation.FindAllString(src, -1) {
		b = strings.TrimSpace(strings.TrimPrefix(b, "--oracall:"))
		var a oracall.Annotation
		if i := strings.IndexByte(b, ' '); i < 0 {
			continue
		} else {
			a.Type, b = string(b[:i]), b[i+1:]
		}
		if i := strings.Index(b, "=>"); i < 0 {
			if i = strings.IndexByte(b, '='); i < 0 {
				a.Name = strings.TrimSpace(b)
			} else {
				a.Name = strings.TrimSpace(b[:i])
				size, err := strconv.Atoi(strings.TrimSpace(b[i+1:]))
				if err != nil {
					return annotations, err
				}
				a.Size = size
			}
		} else {
			a.Name, a.Other = strings.TrimSpace(b[:i]), strings.TrimSpace(b[i+2:])
		}
		annotations = append(annotations, a)
	}
	return annotations, nil
}

func parseDocs(ctx context.Context, text string) (map[string]string, error) {
	// logger := zlog.SFromContext(ctx)
	m := make(map[string]string)
	l := NewLexer(ctx, "docs", text)
	var buf bytes.Buffer
	var err error
	l.Iter(func(it item) bool {
		// logger.Debug("parseDocs", "item", item, "start", l.start, "pos", l.pos, "length", len(l.input))
		switch it.typ {
		case itemError:
			err = errors.New(it.val)
			return false
		case itemEOF:
			return false
		case itemComment:
			buf.WriteString(it.val)
		case itemText:
			ss := rDecl.FindStringSubmatch(it.val)
			if ss != nil {
				m[ss[2]] = buf.String()
			}
			buf.Reset()
		}
		return true
	})
	return m, err
}

// The lexer structure shamelessly copied from
// https://talks.golang.org/2011/lex.slide#22

// stateFn represents the state of the scanner
// as a function that returns the next state.
type stateFn func(*lexer) stateFn

const (
	bcBegin  = "/*"
	bcEnd    = "*/"
	lcBegin  = "--"
	lcEnd    = "\n"
	strBegin = "'"
	//eol     = "\n"
)

type itemType uint8

const (
	itemError itemType = iota
	itemEOF
	itemText
	itemComment
	itemSep
	itemString
)

func (typ itemType) String() string {
	switch typ {
	case itemError:
		return "ERROR"
	case itemEOF:
		return "EOF"
	case itemText:
		return "text"
	case itemComment:
		return "comment"
	case itemSep:
		return "sep"
	case itemString:
		return "string"
	default:
		return "UNKNOWN"
	}
}

func lexText(l *lexer) stateFn {
	if l.pos == len(l.input) {
		l.emit(itemEOF)
		return nil
	}

	// skip --...\n
	what, pos := findFirst(l.input[l.pos:], strBegin, lcBegin, bcBegin)
	var next func(*lexer) stateFn
	switch what {
	case lcBegin:
		next = lexLineComment
	case bcBegin:
		next = lexBlockComment
	case strBegin:
		next = lexString
	}
	if next != nil {
		l.pos += pos
		if l.pos != l.start {
			l.emit(itemText)
		}
		l.pos += len(what)
		l.emit(itemSep)
		l.start = l.pos
		if l.start > l.pos {
			panic(fmt.Errorf("start=%d>pos=%d [pos=%d]", l.start, l.pos, pos))
		}
		return next
	}
	l.pos = len(l.input)
	//logger.Log("msg", "lexText", "start", l.start, "pos", l.pos)
	l.emit(itemText)
	return nil
}

func lexString(l *lexer) stateFn {
	for {
		i := strings.IndexByte(l.input[l.pos:], '\'')
		l.logger.Debug("lexString", "pos", l.pos, "i", i)
		if i < 0 {
			break
		}
		l.pos += i
		if len(l.input[l.pos:]) > 1 && l.input[l.pos+1] == '\'' {
			l.pos += 1
		} else {
			break
		}
	}
	l.emit(itemString)
	l.pos++
	// l.logger.Debug("lexString", "emit", "string", "pos", l.pos, "start", l.start, "length", len(l.input))
	l.emit(itemSep)
	return lexText
}

func lexLineComment(l *lexer) stateFn {
	if l.pos < 0 || len(l.input) <= l.pos {
		l.emit(itemEOF)
		return nil
	}
	end := strings.Index(l.input[l.pos:], lcEnd)
	if end < 0 {
		l.emit(itemError)
		return nil
	}
	l.pos += end
	l.emit(itemComment)
	l.start += len(lcEnd)
	l.pos += len(lcEnd)
	if l.start > l.pos {
		panic("start > pos")
	}
	return lexText
}

func lexBlockComment(l *lexer) stateFn {
	if l.pos < 0 || len(l.input) <= l.pos {
		l.emit(itemEOF)
		return nil
	}
	end := strings.Index(l.input[l.pos:], bcEnd)
	if end < 0 {
		l.emit(itemError)
		return nil
	}
	l.pos += end
	l.emit(itemComment)
	l.pos += len(bcEnd)
	l.emit(itemSep)
	l.pos += len(bcEnd)
	if l.start > l.pos {
		panic("starT > pos")
	}
	return lexText
}

type item struct {
	val string
	typ itemType
}

func (i item) String() string {
	repl := func(s string) string { return s }
	switch i.typ {
	case itemEOF:
		return "EOF"
	case itemError:
		return i.val
	case itemString:
		repl = strings.NewReplacer("''", "'").Replace
	}
	val := repl(i.val)
	if len(val) > 20 {
		if len(val) > 40 {
			return fmt.Sprintf("%.20s...%.20s", val, val[len(val)-20:])
		}
		return fmt.Sprintf("%.20s...", val)
	}
	return val
}

// lexer holds the state of the scanner.
type lexer struct {
	items  chan item // channel of scanned items.
	logger *slog.Logger
	state  stateFn
	name   string // used only for error reports.
	input  string // the string being scanned.
	start  int    // start position of this item.
	pos    int    // current position in the input.
	//width int       // width of last rune read from input.
}

// run lexes the input by executing state functions until
// the state is nil.
/*
func (l *lexer) run() {
	for state := lexText; state != nil; {
		state = state(l)
	}
}
*/

// emit passes an item back to the client.
func (l *lexer) emit(t itemType) {
	if l.state == nil || l.items == nil {
		return
	}
	l.logger.Debug("EMIT", "typ", t, "val", l.input[l.start:l.pos])
	if l.start != l.pos {
		l.items <- item{typ: t, val: l.input[l.start:l.pos]}
		l.start = l.pos
		if t == itemEOF {
			l.start = len(l.input)
		}
	}
	if l.start == len(l.input) {
		l.logger.Debug("EOF")
		l.items <- item{typ: itemEOF}
		close(l.items) // No more tokens will be delivered.
		l.state = nil
	}
}

// Iter iterates over the items, calling yield with all the found items.
func (l *lexer) Iter(yield func(item) bool) {
	for {
		select {
		case it, ok := <-l.items:
			l.logger.Debug("Iter", "it", it, "ok", ok)
			if !ok {
				yield(item{typ: itemEOF})
				return
			}
			if !yield(it) {
				return
			}
		default:
			l.logger.Debug("Iter=default", "state", l.state != nil)
			if l.state == nil {
				yield(item{typ: itemEOF})
				return
			}
			l.state = l.state(l)
		}
	}
}

// Lex creates a new scanner for the input string.
func NewLexer(ctx context.Context, name, input string) *lexer {
	input = strings.TrimRight(input, " \t\v")
	if !strings.HasSuffix(input, "\n") {
		input += "\n"
	}
	l := &lexer{
		name:   name,
		input:  input,
		state:  lexText,
		items:  make(chan item, 3),
		logger: zlog.SFromContext(ctx),
	}
	return l
}

func findFirst(haystack string, needles ...string) (what string, where int) {
	end := len(haystack)
	for _, needle := range needles {
		pos := strings.Index(haystack[:end], needle)
		if pos != -1 {
			what, where = needle, pos
			end = pos
		}
	}
	return what, where
}

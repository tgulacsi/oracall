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
	logger.Debug("parseDocs", "docs", len(docs), "error", err)
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
	logger.Debug("parseDocs", "docs", len(docs), "error", err)
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
	logger := zlog.SFromContext(ctx)
	m := make(map[string]string)
	l := lex(ctx, "docs", text)
	var buf bytes.Buffer
	for {
		item := l.nextItem()
		logger.Debug("parseDocs", "item", item, "start", l.start, "pos", l.pos, "length", len(l.input))
		switch item.typ {
		case itemError:
			return m, errors.New(item.val)
		case itemEOF:
			return m, nil
		case itemComment:
			buf.WriteString(item.val)
		case itemText:
			ss := rDecl.FindStringSubmatch(item.val)
			if ss != nil {
				m[ss[2]] = buf.String()
			}
			buf.Reset()
		}
	}
}

// The lexer structure shamelessly copied from
// https://talks.golang.org/2011/lex.slide#22

// stateFn represents the state of the scanner
// as a function that returns the next state.
type stateFn func(*lexer) stateFn

const (
	bcBegin = "/*"
	bcEnd   = "*/"
	lcBegin = "--"
	lcEnd   = "\n"
	//eol     = "\n"
)

type itemType uint8

const (
	itemError itemType = iota
	itemEOF
	itemText
	itemComment
)

func lexText(l *lexer) stateFn {
	if l.pos == len(l.input) {
		l.emit(itemEOF)
		return nil
	}

	// skip --...\n
	lcPos := strings.Index(l.input[l.pos:], lcBegin)
	bcPos := strings.Index(l.input[l.pos:], bcBegin)
	var pos int
	var next func(*lexer) stateFn
	if lcPos >= 0 && bcPos >= 0 {
		if lcPos < bcPos {
			pos, next = lcPos, lexLineComment
		} else {
			pos, next = bcPos, lexBlockComment
		}
	} else if lcPos >= 0 {
		pos, next = lcPos, lexLineComment
	} else if bcPos >= 0 {
		pos, next = bcPos, lexBlockComment
	}
	if next != nil {
		l.pos += pos + 2
		l.logger.Debug("lexText", "lcPos", lcPos, "bcPos", bcPos, "pos", pos, "l.pos", l.pos, "l.start", l.start)
		l.emit(itemText)
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
	l.start += len(bcEnd)
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
	switch i.typ {
	case itemEOF:
		return "EOF"
	case itemError:
		return i.val
	}
	if len(i.val) > 20 {
		if len(i.val) > 40 {
			return fmt.Sprintf("%.20q...%.20q", i.val, i.val[len(i.val)-20:])
		}
		return fmt.Sprintf("%.20q...", i.val)
	}
	return fmt.Sprintf("%q", i.val)
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
	// logger.Log("EMIT", item{typ: t, val: l.input[l.start:l.pos]})
	l.items <- item{typ: t, val: l.input[l.start:l.pos]}
	l.start = l.pos
	if l.start == len(l.input) {
		l.items <- item{typ: itemEOF}
		close(l.items) // No more tokens will be delivered.
	}
}

// nextItem returns the next item from the input.
func (l *lexer) nextItem() item {
	for {
		select {
		case it := <-l.items:
			return it
		default:
			if l.state == nil {
				return item{typ: itemEOF}
			}
			l.state = l.state(l)
		}
	}
}

// lex creates a new scanner for the input string.
func lex(ctx context.Context, name, input string) *lexer {
	l := &lexer{
		name:   name,
		input:  input,
		state:  lexText,
		items:  make(chan item, 2), // Two items sufficient.
		logger: zlog.SFromContext(ctx),
	}
	return l
}

// next returns the next rune in the input.
/*
func (l *lexer) next() rune {
	if l.pos >= len(l.input) {
		l.width = 0
		return 0x4
	}
	var r rune
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += l.width
	return r
}

// error returns an error token and terminates the scan
// by passing back a nil pointer that will be the next
// state, terminating l.run.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{
		itemError,
		fmt.Sprintf(format, args...),
	}
	return nil
}
*/

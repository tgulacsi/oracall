// Copyright 2017, 2024 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"strings"

	"github.com/UNO-SOFT/zlog/v2"
)

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
	ItemError itemType = iota
	ItemEOF
	ItemText
	ItemComment
	ItemSep
	ItemString
)

func (typ itemType) String() string {
	switch typ {
	case ItemError:
		return "ERROR"
	case ItemEOF:
		return "EOF"
	case ItemText:
		return "text"
	case ItemComment:
		return "comment"
	case ItemSep:
		return "sep"
	case ItemString:
		return "string"
	default:
		return "UNKNOWN"
	}
}

func lexText(l *lexer) stateFn {
	if l.pos == len(l.input) {
		l.emit(ItemEOF)
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
			l.emit(ItemText)
		}
		l.pos += len(what)
		l.emit(ItemSep)
		l.start = l.pos
		if l.start > l.pos {
			panic(fmt.Errorf("start=%d>pos=%d [pos=%d]", l.start, l.pos, pos))
		}
		return next
	}
	l.pos = len(l.input)
	//logger.Log("msg", "lexText", "start", l.start, "pos", l.pos)
	l.emit(ItemText)
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
	l.emit(ItemString)
	l.pos++
	// l.logger.Debug("lexString", "emit", "string", "pos", l.pos, "start", l.start, "length", len(l.input))
	l.emit(ItemSep)
	return lexText
}

func lexLineComment(l *lexer) stateFn {
	if l.pos < 0 || len(l.input) <= l.pos {
		l.emit(ItemEOF)
		return nil
	}
	end := strings.Index(l.input[l.pos:], lcEnd)
	if end < 0 {
		l.emit(ItemError)
		return nil
	}
	l.pos += end
	l.emit(ItemComment)
	l.start += len(lcEnd)
	l.pos += len(lcEnd)
	if l.start > l.pos {
		panic("start > pos")
	}
	return lexText
}

func lexBlockComment(l *lexer) stateFn {
	if l.pos < 0 || len(l.input) <= l.pos {
		l.emit(ItemEOF)
		return nil
	}
	end := strings.Index(l.input[l.pos:], bcEnd)
	if end < 0 {
		l.emit(ItemError)
		return nil
	}
	l.pos += end
	l.emit(ItemComment)
	l.pos += len(bcEnd)
	l.emit(ItemSep)
	l.pos += len(bcEnd)
	if l.start > l.pos {
		panic("starT > pos")
	}
	return lexText
}

type Item struct {
	val string
	typ itemType
}

func (i Item) String() string {
	repl := func(s string) string { return s }
	switch i.typ {
	case ItemEOF:
		return "EOF"
	case ItemError:
		return i.val
	case ItemString:
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
	items  chan Item // channel of scanned items.
	logger *slog.Logger
	state  stateFn
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
		l.items <- Item{typ: t, val: l.input[l.start:l.pos]}
		l.start = l.pos
		if t == ItemEOF {
			l.start = len(l.input)
		}
	}
	if l.start == len(l.input) {
		l.logger.Debug("EOF")
		l.items <- Item{typ: ItemEOF}
		close(l.items) // No more tokens will be delivered.
		l.state = nil
	}
}

// Iter iterates over the items, calling yield with all the found items.
func (l *lexer) Iter(yield func(Item) bool) {
	for {
		select {
		case it, ok := <-l.items:
			l.logger.Debug("Iter", "it", it, "ok", ok)
			if !ok {
				yield(Item{typ: ItemEOF})
				return
			}
			if !yield(it) {
				return
			}
		default:
			l.logger.Debug("Iter=default", "state", l.state != nil)
			if l.state == nil {
				yield(Item{typ: ItemEOF})
				return
			}
			l.state = l.state(l)
		}
	}
}

type ItemsIter iter.Seq[Item]

// Lex creates a new scanner for the input string.
func Lex(ctx context.Context, input string) ItemsIter {
	input = strings.TrimRight(input, " \t\v")
	if !strings.HasSuffix(input, "\n") {
		input += "\n"
	}
	l := &lexer{
		input:  input,
		state:  lexText,
		items:  make(chan Item, 3),
		logger: zlog.SFromContext(ctx),
	}
	return l.Iter
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

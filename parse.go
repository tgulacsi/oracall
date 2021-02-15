/*
Copyright 2017 Tamás Gulácsi

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

func parseDocs(ctx context.Context, text string) (map[string]string, error) {
	m := make(map[string]string)
	l := lex("docs", text)
	var buf bytes.Buffer
	for {
		if err := ctx.Err(); err != nil {
			return m, err
		}
		item := l.nextItem()
		//logger.Log("item", item, "start", l.start, "pos", l.pos, "length", len(l.input))
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

var rDecl = regexp.MustCompile(`(FUNCTION|PROCEDURE) +([^ (;]+)`)

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
	if lcPos >= 0 || bcPos >= 0 {
		pos, next := lcPos, lexLineComment
		if bcPos != -1 && (pos == -1 || pos > bcPos) {
			pos, next = bcPos, lexBlockComment
		}
		l.pos += pos
		//logger.Log("msg", "lexText", "lcPos",lcPos, "bcPos",bcPos,"pos",pos,"l.pos",l.pos, "l.start",l.start)
		l.emit(itemText)
		l.start += 2
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
	items chan item // channel of scanned items.
	state stateFn
	name  string // used only for error reports.
	input string // the string being scanned.
	start int    // start position of this item.
	pos   int    // current position in the input.
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
	//logger.Log("EMIT", item{typ: t, val: l.input[l.start:l.pos]})
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
func lex(name, input string) *lexer {
	l := &lexer{
		name:  name,
		input: input,
		state: lexText,
		items: make(chan item, 2), // Two items sufficient.
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

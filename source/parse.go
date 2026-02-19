// Copyright 2017, 2024 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/UNO-SOFT/zlog/v2"
	oracall "github.com/tgulacsi/oracall/lib"
)

var (
	rDecl   = regexp.MustCompile(`(FUNCTION|PROCEDURE) +([^ (;]+)`)
	rIndent = regexp.MustCompile("^ +")
)

func ParseDocs(ctx context.Context, src string) (docs map[string]string, err error) {
	logger := zlog.SFromContext(ctx)
	docs, _, err = parseDocsAnnotations(ctx, src)
	if err != nil {
		logger.Error("parseDocs", "docs", len(docs), "error", err)
	}
	return docs, err
}

func Parse(ctx context.Context, src string) (docs map[string]string, annotations []oracall.Annotation, err error) {
	logger := zlog.SFromContext(ctx)
	docs, annotations, err = parseDocsAnnotations(ctx, src)
	if err != nil {
		return nil, annotations, err
	}
	if len(annotations) != 0 {
		logger.Debug("found", "annotations", annotations)
		// src = rAnnotation.ReplaceAllString(src, "")
	}
	return docs, annotations, err
}

// var rAnnotation = regexp.MustCompile(`--oracall:(?:(replace(_json)?|rename|tag)\s+[a-zA-Z0-9_#]+\s*=>\s*.+|(handle|private)\s+[a-zA-Z0-9_#]+|max-table-size\s+[a-zA-Z0-9_$]+\s*=\s*[0-9]+)`)

func parseDocsAnnotations(ctx context.Context, src string) (
	docs map[string]string, annotations []oracall.Annotation, err error,
) {
	logger := zlog.SFromContext(ctx)
	docs = make(map[string]string)
	var buf bytes.Buffer
Loop:
	for it := range Lex(ctx, src) {
		// logger.Debug("parseDocs", "item", item, "start", l.start, "pos", l.pos, "length", len(l.input))
		switch it.typ {
		case ItemError:
			err = errors.New(it.val)

		case ItemEOF:
			err = nil
			break Loop

		case ItemText:
			ss := rDecl.FindStringSubmatch(it.val)
			if ss != nil {
				docs[ss[2]] = buf.String()
			} else if s := buf.String(); (strings.Contains(s, "---") || strings.Contains(s, "# ")) &&
				(len(docs) == 0 || len(docs) == 1 && docs[""] != "") {
				indents := rIndent.FindAllStringIndex(s, -1)
				if len(indents) != 0 {
					minIndent := 8
					for _, ii := range indents {
						minIndent = min(minIndent, ii[1]-ii[0])
					}
					if minIndent > 0 {
						indent := strings.Repeat(" ", minIndent)
						s = strings.TrimPrefix(s, indent)
						s = strings.ReplaceAll(s, "\n"+indent, "\n")
					}
				}
				if docs[""] == "" {
					docs[""] = s
				} else {
					docs[""] = docs[""] + "\n\n------\n\n" + s
				}
			}
			buf.Reset()

		case ItemComment:
			var found bool
			if rest, ok := strings.CutPrefix(it.val, "oracall:"); ok {
				a, err := parseAnnotation(rest)
				if err != nil {
					return docs, annotations, err
				} else if a != (oracall.Annotation{}) {
					found = true
					annotations = append(annotations, a)
				}
			}
			if !found {
				if strings.Contains(it.val, "oracall:") {
					logger.Warn("oracall", "val", it.val)
				}
				buf.WriteString(it.val)
			}

		}
	}
	return docs, annotations, err
}

func parseAnnotation(b string) (oracall.Annotation, error) {
	var a oracall.Annotation
	if i := strings.IndexByte(b, ' '); i < 0 {
		return a, nil
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
				return a, err
			}
			a.Size = size
		}
	} else {
		a.Name, a.Other = strings.TrimSpace(b[:i]), strings.TrimSpace(b[i+2:])
	}
	return a, nil
}

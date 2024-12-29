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
	var buf bytes.Buffer
	var err error
	for it := range Lex(ctx, text) {
		// logger.Debug("parseDocs", "item", item, "start", l.start, "pos", l.pos, "length", len(l.input))
		switch it.typ {
		case ItemError:
			return m, errors.New(it.val)
		case ItemEOF:
			return m, nil
		case ItemComment:
			buf.WriteString(it.val)
		case ItemText:
			ss := rDecl.FindStringSubmatch(it.val)
			if ss != nil {
				m[ss[2]] = buf.String()
			}
			buf.Reset()
		}
	}
	return m, err
}

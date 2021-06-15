// Copyright 2019, 2021 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	rBegInput  = regexp.MustCompile("\n\\s*(?:- )?in(?:put)?:? *\n")
	rBegOutput = regexp.MustCompile("\n\\s*(?:(?:- )?out(?:put)?|ret(?:urns?)?):? *\n")
)

func splitDoc(doc string) (common, input, output string) {
	if doc = trimStartEmptyLines(doc); doc == "" {
		return common, input, output
	}
	common = doc
	ii := rBegOutput.FindStringIndex(doc)
	if ii != nil {
		doc, output = doc[:ii[0]], doc[ii[1]:]
	}
	if ii = rBegInput.FindStringIndex(doc); ii != nil {
		common, input = doc[:ii[0]], doc[ii[1]:]
	}
	return common, input, output
}

func splitByOffset(doc string) []string {
	if doc = trimStartEmptyLines(doc); doc == "" {
		return nil
	}
	var last, pos int
	lastOff := firstNotSpace(doc)
	var parts []string
	for _, line := range strings.SplitAfter(doc, "\n") {
		actOff := firstNotSpace(line)
		if lastOff >= actOff {
			if pos > last {
				parts = append(parts, doc[last:pos])
			}
			last = pos
		}
		pos += len(line)
	}
	if pos > last {
		parts = append(parts, doc[last:pos])
	}
	return parts
}

// trimStartEmptyLines the starting empty lines, keep the spaces (offset)
func trimStartEmptyLines(doc string) string {
	if doc = strings.TrimRight(doc, " \n\t"); doc == "" {
		return ""
	}
	if i := firstNotSpace(doc); i >= 0 {
		if j := strings.LastIndexByte(doc[:i], '\n'); j >= 0 {
			return doc[j+1:]
		}
	}
	return doc
}

func getDirDoc(doc string, dirmap direction) argDocs {
	var D argDocs
	common, input, output := splitDoc(doc)
	D.Pre = common
	if dirmap == DIR_IN {
		D.Parse(input)
	} else {
		D.Parse(output)
	}
	return D
}

type argDocs struct {
	Map       map[string]string
	Pre, Post string
	Docs      []string
}

func (D *argDocs) Parse(doc string) {
	D.Docs = splitByOffset(doc)
	for _, line := range D.Docs {
		sline := strings.TrimSpace(line)
		if sline == "" || !(sline[0] == '-' || sline[0] == '*') {
			continue
		}
		sline = strings.TrimLeft(sline[1:], " \t")
		i := strings.IndexAny(sline, "-:")
		if i < 0 {
			continue
		}
		if D.Map == nil {
			D.Map = make(map[string]string)
		}
		D.Map[strings.TrimRight(sline[:i], " \t")] = strings.TrimLeft(sline[i+1:], " \t")
	}
}
func firstNotSpace(doc string) int {
	return strings.IndexFunc(doc, func(r rune) bool { return !unicode.IsSpace(r) })
}

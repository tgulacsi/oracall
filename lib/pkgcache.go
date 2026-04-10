// Copyright 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/UNO-SOFT/zlog/v2"
	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/google/renameio/v2"
	"github.com/klauspost/compress/zstd"
)

type (
	FunctionCache struct {
		Name          string         `json:",omitzero"`
		Documentation string         `json:",omitzero"`
		Arguments     []UserArgument `json:",omitempty"`
	}

	// PackageCache holds everything known about one DB package, serializable to/from JSON.
	PackageCache struct {
		Name          string
		Documentation string       `json:",omitzero"`
		LastDDL       time.Time    `json:",omitzero"`
		Annotations   []Annotation `json:",omitempty"`
		Functions     map[string]FunctionCache
	}
)

// WritePackageCache writes pc to dir/{PackageName}.json atomically.
func WritePackageCache(ctx context.Context, dir string, pc PackageCache) error {
	logger := zlog.SFromContext(ctx)
	fn := filepath.Join(dir, pc.Name+".json.zst")
	fh, err := renameio.NewPendingFile(fn, renameio.WithPermissions(0640))
	if err != nil {
		logger.Error("create", "file", fn, "error", err)
		return err
	}
	defer fh.Cleanup()
	zw, err := zstd.NewWriter(fh)
	if err != nil {
		return err
	}
	defer zw.Close()
	logger.Debug("write", "file", fn)
	if err := json.MarshalWrite(zw, pc, jsontext.WithIndent("  ")); err != nil {
		logger.Error("marshal", "pc", pc, "error", err)
		return err
	}
	if err = zw.Close(); err != nil {
		return err
	}
	return fh.CloseAtomicallyReplace()
}

// ReadPackageCache reads dir/{pkgName}.json.
func ReadPackageCache(ctx context.Context, dir, pkgName string) (PackageCache, error) {
	logger := zlog.SFromContext(ctx)
	fn := filepath.Join(dir, pkgName+".json.zst")
	logger.Debug("read", "file", fn)
	fh, err := os.Open(fn)
	if err != nil {
		logger.Error("open", "file", "error", err)
		return PackageCache{}, err
	}
	defer fh.Close()
	zr, err := zstd.NewReader(fh)
	if err != nil {
		return PackageCache{}, err
	}
	defer zr.Close()
	var pc PackageCache
	if err = json.UnmarshalRead(zr, &pc); err != nil {
		logger.Error("unmarshal", "file", fn, "error", err)
	}
	return pc, err
}

// ListPackageCaches returns all package names found as .json files in dir.
func ListPackageCaches(dir string) ([]string, error) {
	dis, err := os.ReadDir(dir)
	if err != nil && len(dis) == 0 {
		return nil, err
	}
	var names []string
	for _, di := range dis {
		if di.IsDir() {
			continue
		}
		if bn, ok := strings.CutSuffix(di.Name(), ".json.zst"); ok {
			names = append(names, bn)
		}
	}
	logger.Debug("ListPackageCaches", "dir", dir, "names", names)
	return names, nil
}

// ParsePackageCaches reads all .json files from dir, parses them into Functions.
func ParsePackageCaches(ctx context.Context, dir string, filter func(string) bool) (
	packages map[string]string,
	functions []Function,
	annotations []Annotation,
	err error,
) {
	pkgNames, err := ListPackageCaches(dir)
	if err != nil {
		logger.Error("ListPackageCaches", "error", err)
		return nil, nil, nil, err
	}
	packages = make(map[string]string, len(pkgNames))
	for _, pkgName := range pkgNames {
		pc, err := ReadPackageCache(ctx, dir, pkgName)
		if err != nil {
			logger.Error("ReadPackageCache", "dir", dir, "pkg", pkgName, "error", err)
			return packages, functions, annotations, err
		}
		if pc.Documentation != "" {
			packages[pc.Name] = pc.Documentation
		}
		annotations = append(annotations, pc.Annotations...)
		fns := ParseArgumentsIter(
			FilterAndGroupIter(func(yield func(UserArgument) bool) {
				for _, f := range pc.Functions {
					for _, ua := range f.Arguments {
						if !yield(ua) {
							return
						}
					}
				}
			},
				filter),
			filter,
		)
		// Attach per-function docs.
		for i, f := range fns {
			if f.Documentation == "" {
				if d := pc.Functions[f.Name()].Documentation; d != "" {
					fns[i].Documentation = d
				}
			}
		}
		functions = append(functions, fns...)
		logger.Debug("parse", "pkg", pkgName, "functions", functions)
	}
	return packages, functions, annotations, nil
}

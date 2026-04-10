// Copyright 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: Apache-2.0

package oracall

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/google/renameio/v2"
)

// PackageCache holds everything known about one DB package, serializable to/from JSON.
type PackageCache struct {
	FunctionDocs  map[string]string `json:",omitempty"`
	PackageName   string
	Documentation string         `json:",omitempty"`
	LastDDL       time.Time      `json:",omitzero"`
	Annotations   []Annotation   `json:",omitempty"`
	Arguments     []UserArgument `json:",omitempty"`
}

// WritePackageCache writes pc to dir/{PackageName}.json atomically.
func WritePackageCache(dir string, pc PackageCache) error {
	fn := filepath.Join(dir, pc.PackageName+".json")
	pf, err := renameio.NewPendingFile(fn)
	if err != nil {
		return err
	}
	defer pf.Cleanup()
	enc := jsontext.NewEncoder(pf, jsontext.WithIndent("  "))
	if err := json.MarshalEncode(enc, pc); err != nil {
		return err
	}
	return pf.CloseAtomicallyReplace()
}

// ReadPackageCache reads dir/{pkgName}.json.
func ReadPackageCache(dir, pkgName string) (PackageCache, error) {
	fn := filepath.Join(dir, pkgName+".json")
	fh, err := os.Open(fn)
	if err != nil {
		return PackageCache{}, err
	}
	defer fh.Close()
	var pc PackageCache
	err = json.UnmarshalRead(fh, &pc)
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
		if bn, ok := strings.CutSuffix(di.Name(), ".json"); ok {
			names = append(names, bn)
		}
	}
	return names, nil
}

// ParsePackageCaches reads all .json files from dir, parses them into Functions.
func ParsePackageCaches(dir string, filter func(string) bool) (
	packages map[string]string,
	functions []Function,
	annotations []Annotation,
	err error,
) {
	pkgNames, err := ListPackageCaches(dir)
	if err != nil {
		return nil, nil, nil, err
	}
	packages = make(map[string]string, len(pkgNames))
	for _, pkgName := range pkgNames {
		pc, err := ReadPackageCache(dir, pkgName)
		if err != nil {
			return packages, functions, annotations, err
		}
		if pc.Documentation != "" {
			packages[pc.PackageName] = pc.Documentation
		}
		annotations = append(annotations, pc.Annotations...)
		fns := ParseArgumentsIter(
			FilterAndGroupIter(slices.Values(pc.Arguments), filter),
			filter,
		)
		// Attach per-function docs.
		for i, f := range fns {
			if f.Documentation == "" {
				if d := pc.FunctionDocs[f.Name()]; d != "" {
					fns[i].Documentation = d
				}
			}
		}
		functions = append(functions, fns...)
	}
	return packages, functions, annotations, nil
}

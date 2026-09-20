package models

import (
	"slices"
	"strings"
)

// ServiceKey returns the canonical store key for a service, matching the
// Redis data model (service:<ns>:<name>).
func ServiceKey(ns, name string) string {
	return ServicePrefix + ns + ":" + name
}

// SplitKey extracts the namespace and name from a store key of the form
// <prefix><ns>:<name>, regardless of which prefix it carries.
func SplitKey(key string) (ns, name string) {
	rest := key
	if i := strings.Index(rest, ":"); i >= 0 {
		rest = rest[i+1:]
	}
	if i := strings.Index(rest, ":"); i >= 0 {
		return rest[:i], rest[i+1:]
	}
	return "", rest
}

// Service is a discovered route annotated dashboard metadata.
type Service struct {
	Key   string // service:<ns>:<name>
	NS    string
	Name  string
	URL   string
	Icon  string
	Order int
	Group string
}

// SortServices orders services by Order ascending, then Name for
// determinism. Unannotated services default to the highest order so they
// sink to the end.
func SortServices(svcs []Service) {
	slices.SortFunc(svcs, func(a, b Service) int {
		switch {
		case a.Order < b.Order:
			return -1
		case a.Order > b.Order:
			return 1
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		}
		return 0
	})
}

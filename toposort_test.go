// Copyright 2018 GRAIL, Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package infra

import "testing"

func TestTopoSorter(t *testing.T) {
	var (
		credentials = &instance{name: "credentials"}
		repository  = &instance{name: "repository"}
		cluster     = &instance{name: "cluster"}
		database    = &instance{name: "database"}
		orphan      = &instance{name: "orphan"}
	)
	graph := make(topoSorter)
	graph.Add(database, credentials)
	graph.Add(database, repository)
	graph.Add(cluster, credentials)
	graph.Add(repository, credentials)
	graph.Add(orphan, nil)

	order := graph.Sort()
	var seen []*instance
	for from, tos := range graph {
		seen = append(seen, from)
		i := index(from, order)
		for _, to := range tos {
			j := index(to, order)
			if i <= j {
				t.Errorf("invalid order: %d <= %d (%v)", i, j, order)
			}
		}
	}
	if got, want := len(seen), len(graph); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	for _, inst := range seen {
		delete(graph, inst)
	}
	if len(graph) != 0 {
		t.Error("order mismatch")
	}
}

func index(needle *instance, haystack []*instance) int {
	for i, inst := range haystack {
		if needle == inst {
			return i
		}
	}
	panic("not found")
}

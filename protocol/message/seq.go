// Copyright 2025 Outreach Corporation. All Rights Reserved.

// Description:

// Package message:
package message

import (
	"iter"
)

func CachedSeq2[K, V any](seq iter.Seq2[K, V]) iter.Seq2[K, V] {
	type pair struct {
		k K
		v V
	}

	pairs := make([]pair, 0, 8)
	index := 0

	s := func(yield func(K, V) bool) {
		for k, v := range seq {
			index += 1
			pairs = append(pairs, pair{k, v})
			ok := yield(k, v)
			if !ok {
				return
			}
		}
	}

	return s

}

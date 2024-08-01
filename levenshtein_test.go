package main

import "testing"

type levenshteinTest struct {
	a string
	b string
	d int
}

func TestLevenshtein(t *testing.T) {
	inputs := []levenshteinTest{
		{"kitten", "sitting", 3},
		{"kitten", "kinder", 3},
		{"kitten", "kind", 4},
		{"cool", "coil", 1},
		{"tool", "too", 1},
	}
	var res int
	for _, input := range inputs {
		res = LevenshteinDistance(input.a, input.b)
		if res != input.d {
			t.Errorf(
				"distance %d different from expected %d between %s and %s",
				res,
				input.d,
				input.a,
				input.b,
			)
		}
	}
}

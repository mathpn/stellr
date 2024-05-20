package main

import (
	"fmt"
	"strings"
	"unicode"
)

var corpus = []string{
	"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
	"Orci sagittis eu volutpat odio facilisis mauris sit.",
	"Duis ut diam quam nulla porttitor massa id neque.",
	"Cursus vitae congue mauris rhoncus aenean vel elit scelerisque mauris.",
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

type MutableTextIndex interface {
	Search(query string, tokenizer func(string) []string) []int
	Add(tokens []string, id int)
}

type hashmapIndex map[string][]int

func (index hashmapIndex) Search(query string, tokenizer func(string) []string) []int {
	var r []int
	for _, token := range tokenizer(query) {
		if ids, ok := index[token]; ok {
			if r == nil {
				r = ids
			} else {
				r = intersection(r, ids)
			}
		} else {
			return nil
		}
	}
	return r
}

func (index hashmapIndex) Add(tokens []string, id int) {
	for _, token := range tokens {
		index[token] = append(index[token], id)
	}
}

func intersection(a []int, b []int) []int {
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	r := make([]int, 0, maxLen)
	var i, j int
	for i < len(a) && j < len(b) {
		if a[i] < b[j] {
			i++
		} else if a[i] > b[j] {
			j++
		} else {
			r = append(r, a[i])
			i++
			j++
		}
	}
	return r
}

func main() {
	tokenized_corpus := make([][]string, 0)
	for _, text := range corpus {
		tokenized_corpus = append(tokenized_corpus, tokenize(text))
	}

	index := make(hashmapIndex)
	for i, tokens := range tokenized_corpus {
		index.Add(tokens, i)
	}
	matching_ids := index.Search("ut", tokenize)
	for _, id := range matching_ids {
		fmt.Println(corpus[id])
	}
}

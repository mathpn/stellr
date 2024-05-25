package main

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/RoaringBitmap/roaring"
)

var corpus = []string{
	"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
	"Orci sagittis eu volutpat odio facilisis mauris sit.",
	"Duis ut diam quam nulla porttitor massa id neque.",
	"Cursus vitae congue mauris rhoncus aenean vel elit scelerisque mauris.",
	"Cursus ut ut ut vitae congue mauris rhoncus aenean vel elit scelerisque mauris.",
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

type hashmapIndex map[string]*roaring.Bitmap

func (index hashmapIndex) Search(query string, tokenizer func(string) []string) []uint32 {
	var r *roaring.Bitmap
	for _, token := range tokenizer(query) {
		if bitmap, ok := index[token]; ok {
			if r == nil {
				r = bitmap
			} else {
				r.And(bitmap)
			}
		} else {
			return nil
		}
	}
	return r.ToArray()
}

func Rank(query string, docIds []uint32) []uint32 {
	query_tokens := tokenize(query)
	termFreqs := getTermFrequency(query_tokens)
	scores := make([]float64, len(docIds))

	var doc string
	var refCount float64
	for i, id := range docIds {
		doc = corpus[id]
		docTermFreqs := getTermFrequency(tokenize(doc))
		for token, value := range termFreqs {
			refCount = docTermFreqs[token]
			scores[i] += value * refCount
		}
	}

	sort.Slice(docIds, func(i, j int) bool {
		return scores[i] > scores[j] // descending order
	})
	return docIds
}

func getTermFrequency(tokens []string) map[string]float64 {
	termCounts := make(map[string]int)
	nTokens := float64(len(tokens))
	for _, token := range tokens {
		termCounts[token]++
	}
	termFreqs := make(map[string]float64, len(termCounts))
	for token, count := range termCounts {
		termFreqs[token] = float64(count) / nTokens
	}
	return termFreqs
}

func (index hashmapIndex) Add(tokens []string, id int) {
	for _, token := range tokens {
		bitmap := index[token]
		if bitmap == nil {
			index[token] = roaring.New()
		}
		index[token].Add(uint32(id))
	}
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
	matching_ids = Rank("ut", matching_ids)
	for _, id := range matching_ids {
		fmt.Println(corpus[id])
	}
}

package main

import (
	"fmt"
	"math"
	"os"
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
	Search(query string, tokenizer func(string) []string) []uint32
	Add(tokens []string, id uint32)
	Rank(query string, docIds []uint32, tokenizer func(string) []string) []uint32
}

type hashmapIndex struct {
	invIndex         map[string]*roaring.Bitmap
	wordFreqArray    []map[string]float64
	squaredNormArray []float64
}

func NewHashmapIndex() MutableTextIndex {
	return &hashmapIndex{
		invIndex:         make(map[string]*roaring.Bitmap),
		wordFreqArray:    make([]map[string]float64, 0),
		squaredNormArray: make([]float64, 0),
	}
}

func (index *hashmapIndex) Search(query string, tokenizer func(string) []string) []uint32 {
	var r *roaring.Bitmap
	for _, token := range tokenizer(query) {
		fmt.Println(token)
		if bitmap, ok := index.invIndex[token]; ok {
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

func computeNorm(termFreqs map[string]float64) float64 {
	var norm float64
	for token, queryCount := range termFreqs {
		termFreqs[token] = queryCount
		norm += queryCount * queryCount
	}
	return norm
}

func (index *hashmapIndex) Rank(query string, docIds []uint32, tokenizer func(string) []string) []uint32 {
	query_tokens := tokenizer(query)
	termFreqs := getTermFrequency(query_tokens)
	scores := make([]float64, len(docIds))
	queryNorm := computeNorm(termFreqs)

	var refCount, norm, invNorm float64
	var docTermFreqs map[string]float64
	for i, id := range docIds {
		docTermFreqs = index.wordFreqArray[id]
		norm = index.squaredNormArray[id]
		for token, value := range termFreqs {
			refCount = docTermFreqs[token]
			scores[i] += value * refCount
		}

		invNorm = 1 / math.Sqrt(queryNorm*norm+1e-8)
		scores[i] = math.Sqrt(scores[i] * invNorm)
	}

	// XXX
	for i, id := range docIds {
		fmt.Printf("%.2f -> %s\n", scores[i], corpus[id])
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

func (index *hashmapIndex) Add(tokens []string, id uint32) {
	for _, token := range tokens {
		bitmap := index.invIndex[token]
		if bitmap == nil {
			index.invIndex[token] = roaring.New()
		}
		index.invIndex[token].Add(id)
	}

	termFreqs := getTermFrequency(tokens)
	index.wordFreqArray = append(index.wordFreqArray, termFreqs)
	index.squaredNormArray = append(index.squaredNormArray, computeNorm(termFreqs))
}

func main() {
	tokenized_corpus := make([][]string, 0)
	for _, text := range corpus {
		tokenized_corpus = append(tokenized_corpus, tokenize(text))
	}

	index := NewHashmapIndex()
	for i, tokens := range tokenized_corpus {
		index.Add(tokens, uint32(i))
	}

	query := os.Args[1]
	matching_ids := index.Search(query, tokenize)
	matching_ids = index.Rank(query, matching_ids, tokenize)
	for _, id := range matching_ids {
		fmt.Println(corpus[id])
	}
}

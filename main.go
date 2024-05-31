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

type IndexBuilder interface {
	Add(tokens []string, id uint32)
	Build() SearchIndex
}

type SearchIndex interface {
	Search(query string, tokenizer func(string) []string) []uint32
	Rank(query string, docIds []uint32, tokenizer func(string) []string) []uint32
}

type hashmapIndexBuilder struct {
	invIndex      map[string]*roaring.Bitmap
	docCount      map[string]uint32
	wordFreqArray []map[string]float64
}

type hashmapSearchIndex struct {
	invIndex   map[string]*roaring.Bitmap
	idf        map[string]float64
	tfIdfArray []map[string]float64
	normArray  []float64
	defaultIdf float64
}

func NewHashmapIndex() IndexBuilder {
	return &hashmapIndexBuilder{
		invIndex:      make(map[string]*roaring.Bitmap),
		docCount:      make(map[string]uint32),
		wordFreqArray: make([]map[string]float64, 0),
	}
}

func (index *hashmapSearchIndex) Search(query string, tokenizer func(string) []string) []uint32 {
	var r *roaring.Bitmap
	for _, token := range tokenizer(query) {
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

func computeNorm(tfIdf map[string]float64) float64 {
	var norm float64
	for _, queryCount := range tfIdf {
		norm += queryCount * queryCount
	}
	return norm
}

func (index *hashmapSearchIndex) Rank(query string, docIds []uint32, tokenizer func(string) []string) []uint32 {
	query_tokens := tokenizer(query)
	termFreqs := getTermFrequency(query_tokens)
	scores := make([]float64, len(docIds))

	var refCount, norm, invNorm, queryNorm float64
	var docTermFreqs map[string]float64
	for i, id := range docIds {
		docTermFreqs = index.tfIdfArray[id]
		norm = computeNorm(docTermFreqs) // XXX
		for token, value := range termFreqs {
			tokenIdf, ok := index.idf[token]
			if !ok {
				tokenIdf = index.defaultIdf
			}
			refCount = docTermFreqs[token]
			scores[i] += value * refCount * tokenIdf
			queryNorm += value * value * tokenIdf * tokenIdf
		}

		invNorm = 1 / math.Sqrt(queryNorm*norm+1e-8)
		scores[i] = scores[i] * invNorm
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

func (index *hashmapIndexBuilder) Add(tokens []string, id uint32) {
	for _, token := range tokens {
		bitmap := index.invIndex[token]
		if bitmap == nil {
			index.invIndex[token] = roaring.New()
		}
		index.invIndex[token].Add(id)
	}

	termFreqs := getTermFrequency(tokens)
	for token := range termFreqs {
		index.docCount[token]++
	}
	index.wordFreqArray = append(index.wordFreqArray, termFreqs)
}

func (index *hashmapIndexBuilder) Build() SearchIndex {
	idf := make(map[string]float64, 0)
	nDocs := len(index.wordFreqArray)

	for token, count := range index.docCount {
		idf[token] = math.Log(float64(nDocs) / float64(count))
	}

	tfIdfArray := make([]map[string]float64, len(index.wordFreqArray))
	for i, wordFreq := range index.wordFreqArray {
		for token, freq := range wordFreq {
			tokenIdf, ok := idf[token]
			if !ok {
				panic("oh no") // XXX
			}
			wordFreq[token] = freq * tokenIdf * tokenIdf
		}
		tfIdfArray[i] = wordFreq
	}

	return &hashmapSearchIndex{
		invIndex:   index.invIndex,
		idf:        idf,
		tfIdfArray: index.wordFreqArray,
		defaultIdf: math.Log(1 / float64(nDocs+1)),
	}
}

func main() {
	tokenized_corpus := make([][]string, 0)
	for _, text := range corpus {
		tokenized_corpus = append(tokenized_corpus, tokenize(text))
	}

	indexBuilder := NewHashmapIndex()
	for i, tokens := range tokenized_corpus {
		indexBuilder.Add(tokens, uint32(i))
	}

	query := os.Args[1]
	index := indexBuilder.Build()
	matching_ids := index.Search(query, tokenize)
	matching_ids = index.Rank(query, matching_ids, tokenize)
	for _, id := range matching_ids {
		fmt.Println(corpus[id])
	}
}

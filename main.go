package main

import (
	"bufio"
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
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r)
	})
}

type IndexBuilder interface {
	Add(tokens []string, id uint32)
	Build() SearchIndex
}

type SearchIndex interface {
	Search(query string, tokenizer func(string) []string) *IndexResult
	Rank(tokens []string, docIds []uint32) []uint32
}

type hashmapIndexBuilder struct {
	invIndex      map[string]*roaring.Bitmap
	wordFreqArray []map[string]float64
}

type trieIndexBuilder struct {
	invIndex      *PatriciaTrie
	wordFreqArray []map[string]float64
}

type docEntry struct {
	tfIdf map[string]float64
	norm  float64
}

type hashmapSearchIndex struct {
	invIndex   map[string]*roaring.Bitmap
	idf        map[string]float64
	docEntries []*docEntry
	defaultIdf float64
}

type trieSearchIndex struct {
	invIndex   *PatriciaTrie
	idf        map[string]float64
	docEntries []*docEntry
	defaultIdf float64
}

// Rank implements SearchIndex.
func (t *trieSearchIndex) Rank(tokens []string, docIds []uint32) []uint32 {
	termFreqs := getTermFrequency(tokens)
	scores := make([]float64, len(docIds))

	var refCount, invNorm, queryNorm float64
	var doc *docEntry
	for i, id := range docIds {
		doc = t.docEntries[id]
		for token, value := range termFreqs {
			tokenIdf, ok := t.idf[token]
			if !ok {
				tokenIdf = t.defaultIdf
			}
			refCount = doc.tfIdf[token]
			scores[i] += value * refCount * tokenIdf
			queryNorm += value * value * tokenIdf * tokenIdf
		}

		invNorm = 1 / math.Sqrt(queryNorm*doc.norm+1e-8)
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

func (t *trieSearchIndex) Search(query string, tokenizer func(string) []string) *IndexResult {
	var res *IndexResult
	r := &IndexResult{set: roaring.New(), tokens: make([]string, 0)}
	for _, token := range tokenizer(query) {
		if res = t.invIndex.StartsWith(token); res != nil {
			r.Combine(res)
		}
	}
	return r
}

func NewHashmapIndex() IndexBuilder {
	return &hashmapIndexBuilder{
		invIndex:      make(map[string]*roaring.Bitmap),
		wordFreqArray: make([]map[string]float64, 0),
	}
}

func NewTrieIndex() IndexBuilder {
	return &trieIndexBuilder{
		invIndex:      NewPatriciaTrie(),
		wordFreqArray: make([]map[string]float64, 0),
	}
}

func (index *hashmapSearchIndex) Search(query string, tokenizer func(string) []string) *IndexResult {
	var r *roaring.Bitmap
	tokens := tokenizer(query)
	for _, token := range tokens {
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
	return &IndexResult{set: r, tokens: tokens}
}

func computeNorm(tfIdf map[string]float64) float64 {
	var norm float64
	for _, queryCount := range tfIdf {
		norm += queryCount * queryCount
	}
	return norm
}

func (index *hashmapSearchIndex) Rank(tokens []string, docIds []uint32) []uint32 {
	termFreqs := getTermFrequency(tokens)
	scores := make([]float64, len(docIds))

	var refCount, invNorm, queryNorm float64
	var doc *docEntry
	for i, id := range docIds {
		doc = index.docEntries[id]
		for token, value := range termFreqs {
			tokenIdf, ok := index.idf[token]
			if !ok {
				tokenIdf = index.defaultIdf
			}
			refCount = doc.tfIdf[token]
			scores[i] += value * refCount * tokenIdf
			queryNorm += value * value * tokenIdf * tokenIdf
		}

		invNorm = 1 / math.Sqrt(queryNorm*doc.norm+1e-8)
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
	index.wordFreqArray = append(index.wordFreqArray, termFreqs)
}

func (index *trieIndexBuilder) Add(tokens []string, id uint32) {
	var bitmap *roaring.Bitmap
	for _, token := range tokens {
		bitmap = index.invIndex.Search(token)
		if bitmap == nil {
			bitmap = roaring.New()
		}
		bitmap.Add(id)
		index.invIndex.Insert(token, bitmap) // XXX replace if exists
	}

	termFreqs := getTermFrequency(tokens)
	index.wordFreqArray = append(index.wordFreqArray, termFreqs)
}

func (index *hashmapIndexBuilder) Build() SearchIndex {
	idf := make(map[string]float64, 0)
	nDocs := len(index.wordFreqArray)

	for token, set := range index.invIndex {
		idf[token] = math.Log(float64(nDocs) / float64(set.GetCardinality()))
	}

	docEntries := make([]*docEntry, len(index.wordFreqArray))
	var doc *docEntry
	for i, wordFreq := range index.wordFreqArray {
		doc = &docEntry{}
		for token, freq := range wordFreq {
			tokenIdf, ok := idf[token]
			if !ok {
				panic("oh no") // XXX
			}
			wordFreq[token] = freq * tokenIdf * tokenIdf
		}

		doc.tfIdf = wordFreq
		doc.norm = computeNorm(doc.tfIdf)

		docEntries[i] = doc
	}

	return &hashmapSearchIndex{
		invIndex:   index.invIndex,
		idf:        idf,
		docEntries: docEntries,
		defaultIdf: math.Log(1 / float64(nDocs+1)),
	}
}

func (index *trieIndexBuilder) Build() SearchIndex {
	idf := make(map[string]float64, 0)
	nDocs := len(index.wordFreqArray)

	tokenSets := index.invIndex.Traversal()
	var cardinality uint64
	for _, tokenSet := range tokenSets {
		cardinality = tokenSet.set.GetCardinality()
		idf[tokenSet.token] = math.Log(float64(nDocs) / float64(cardinality))
	}

	docEntries := make([]*docEntry, len(index.wordFreqArray))
	var doc *docEntry
	for i, wordFreq := range index.wordFreqArray {
		doc = &docEntry{}
		for token, freq := range wordFreq {
			tokenIdf, ok := idf[token]
			if !ok {
				panic("oh no") // XXX
			}
			wordFreq[token] = freq * tokenIdf * tokenIdf
		}
		doc.tfIdf = wordFreq
		doc.norm = computeNorm(doc.tfIdf)

		docEntries[i] = doc
	}

	return &trieSearchIndex{
		invIndex:   index.invIndex,
		idf:        idf,
		docEntries: docEntries,
		defaultIdf: math.Log(1 / float64(nDocs+1)),
	}
}

func ReadCorpus(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

func main() {
	corpusPath := os.Args[1]

	corpus, err := ReadCorpus(corpusPath)
	if err != nil {
		panic(err)
	}

	tokenized_corpus := make([][]string, 0)
	for _, text := range corpus {
		tokenized_corpus = append(tokenized_corpus, tokenize(text))
	}

	indexBuilder := NewTrieIndex()
	// indexBuilder := NewHashmapIndex()
	for i, tokens := range tokenized_corpus {
		indexBuilder.Add(tokens, uint32(i))
	}

	query := os.Args[2]
	index := indexBuilder.Build()

	searchResult := index.Search(query, tokenize)
	matching_ids := index.Rank(searchResult.tokens, searchResult.set.ToArray())
	for _, id := range matching_ids {
		fmt.Println(corpus[id])
	}
}

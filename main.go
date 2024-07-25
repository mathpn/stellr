package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"unicode"

	"github.com/RoaringBitmap/roaring"
)

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
	Rank(tokens []string, docIds []uint32) []RankResult
}

type RankResult struct {
	id    uint32
	score float64
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

func (t *trieSearchIndex) Rank(tokens []string, docIds []uint32) []RankResult {
	termFreqs := getTermFrequency(tokens)
	result := make([]RankResult, len(docIds))

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
			result[i].id = id
			result[i].score += value * refCount * tokenIdf
			queryNorm += value * value * tokenIdf * tokenIdf
		}

		invNorm = 1 / math.Sqrt(queryNorm*doc.norm+1e-8)
		result[i].score = result[i].score * invNorm
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].score > result[j].score // descending order
	})
	return result
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

func (index *hashmapSearchIndex) Rank(tokens []string, docIds []uint32) []RankResult {
	termFreqs := getTermFrequency(tokens)
	result := make([]RankResult, len(docIds))

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
			result[i].id = id
			result[i].score += value * refCount * tokenIdf
			queryNorm += value * value * tokenIdf * tokenIdf
		}

		invNorm = 1 / math.Sqrt(queryNorm*doc.norm+1e-8)
		result[i].score = result[i].score * invNorm
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].score > result[j].score // descending order
	})
	return result
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

type App struct {
	indexBuilder IndexBuilder
	index        SearchIndex
	corpus       []string
}

func (a *App) uploadCorpus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20) // 10 MB
	if err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	file, fileHeader, err := r.FormFile("corpus")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	var tokenizedLine []string
	a.indexBuilder = NewTrieIndex()
	scanner := bufio.NewScanner(file)
	i := 0
	for scanner.Scan() {
		line := scanner.Text()
		tokenizedLine = tokenize(line)
		a.indexBuilder.Add(tokenizedLine, uint32(i))
		a.corpus = append(a.corpus, line)
		i++
	}

	if err := scanner.Err(); err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Uploaded File: %+v\n", fileHeader.Filename)
	fmt.Printf("File Size: %+v\n", fileHeader.Size)
	fmt.Printf("MIME Header: %+v\n", fileHeader.Header)

	fmt.Fprint(w, "creating index brrr\n")
	a.index = a.indexBuilder.Build()
}

type searchResponse struct {
	Text  string  `json:"text"`
	Score float64 `json:"score"`
	Id    uint32  `json:"id"`
}

func (a *App) search(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	query := r.URL.Query().Get("query")

	searchResult := a.index.Search(query, tokenize)
	matching_ids := a.index.Rank(searchResult.tokens, searchResult.set.ToArray())
	result := make([]searchResponse, 0)

	var response searchResponse
	for _, res := range matching_ids {
		response = searchResponse{Id: res.id, Score: math.Round(1000 * res.score), Text: a.corpus[res.id]}
		result = append(result, response)
	}

	err := json.NewEncoder(w).Encode(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func main() {
	app := &App{corpus: make([]string, 0)}

	http.HandleFunc("/uploadCorpus", app.uploadCorpus)
	http.HandleFunc("/search", app.search)
	http.ListenAndServe(":8345", nil)
}

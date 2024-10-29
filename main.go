package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/RoaringBitmap/roaring"
	"github.com/kljensen/snowball"
	"github.com/kljensen/snowball/english"
	"github.com/kljensen/snowball/french"
	"github.com/kljensen/snowball/hungarian"
	"github.com/kljensen/snowball/norwegian"
	"github.com/kljensen/snowball/russian"
	"github.com/kljensen/snowball/spanish"
	"github.com/kljensen/snowball/swedish"
)

const maxLineSize = 1 << 20 // 1 MB

type (
	SearchType int
	Operator   int
)

const (
	ExactSearch SearchType = iota
	PrefixSearch
	FuzzySearch
)

const (
	Or Operator = iota
	And
)

func tokenize(text string) []string {
	text = strings.ToLower(text)
	tokens := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r)
	})
	return tokens
}

func filterStopWords(tokens []string, language string) []string {
	stopWordFuncs := map[string]func(string) bool{
		"english":   english.IsStopWord,
		"french":    french.IsStopWord,
		"hungarian": hungarian.IsStopWord,
		"norwegian": norwegian.IsStopWord,
		"russian":   russian.IsStopWord,
		"spanish":   spanish.IsStopWord,
		"swedish":   swedish.IsStopWord,
	}

	isStopWord, ok := stopWordFuncs[language]
	if !ok {
		return tokens
	}

	var result []string
	for _, token := range tokens {
		if !isStopWord(token) {
			result = append(result, token)
		}
	}
	return result
}

func stemTokens(tokens []string, language string) ([]string, error) {
	for i, token := range tokens {
		stemmed, err := snowball.Stem(token, language, false)
		if err != nil {
			return nil, err
		}
		tokens[i] = stemmed
	}
	return tokens, nil
}

type IndexBuilder interface {
	Add(tokens []string, id uint32)
	Build() SearchIndex
}

type SearchIndex interface {
	Search(query string, searchType SearchType, operator Operator, distance int) *IndexResult
	Rank(tokens []string, docIds []uint32) []RankResult
}

type RankResult struct {
	id    uint32
	score float64
}

type trieIndexBuilder struct {
	invIndex      *PatriciaTrie
	wordFreqArray []map[string]float64
}

type docEntry struct {
	tfIdf map[string]float64
	norm  float64
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

	var doc *docEntry
	for i, id := range docIds {
		var refValue, invNorm, queryNorm float64
		doc = t.docEntries[id]
		for token, value := range termFreqs {
			tokenIdf, ok := t.idf[token]
			if !ok {
				tokenIdf = t.defaultIdf
			}
			refValue = doc.tfIdf[token]
			result[i].id = id
			result[i].score += value * tokenIdf * refValue
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

func (t *trieSearchIndex) Search(
	query string, searchType SearchType, operator Operator, distance int,
) *IndexResult {
	var searchFn func(key string) *IndexResult

	switch searchType {
	case ExactSearch:
		searchFn = t.invIndex.Search
	case PrefixSearch:
		searchFn = t.invIndex.StartsWith
	case FuzzySearch:
		searchFn = func(key string) *IndexResult { return t.invIndex.FuzzySearch(key, distance) }
	}

	var res *IndexResult
	r := &IndexResult{set: nil, tokens: make([]string, 0)}

	var combineFn func(res *IndexResult)
	if operator == And {
		combineFn = r.CombineAnd
	} else {
		combineFn = r.CombineOr
	}
	for _, token := range tokenize(query) {
		if res = searchFn(token); res != nil {
			combineFn(res)
		}
	}
	return r
}

func NewTrieIndex() IndexBuilder {
	return &trieIndexBuilder{
		invIndex:      NewPatriciaTrie(),
		wordFreqArray: make([]map[string]float64, 0),
	}
}

func computeNorm(tfIdf map[string]float64) float64 {
	var norm float64
	for _, value := range tfIdf {
		norm += value * value
	}
	return norm
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

func (index *trieIndexBuilder) Add(tokens []string, id uint32) {
	var result *IndexResult
	var set *roaring.Bitmap
	for _, token := range tokens {
		result = index.invIndex.Search(token)
		if result == nil {
			set = roaring.New()
		} else {
			set = result.set
		}
		set.Add(id)
		index.invIndex.Insert(token, set)
	}

	termFreqs := getTermFrequency(tokens)
	index.wordFreqArray = append(index.wordFreqArray, termFreqs)
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
				panic("error: no IDF found")
			}
			wordFreq[token] = freq * tokenIdf
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

type App struct {
	indexBuilder IndexBuilder
	index        SearchIndex
	corpus       []string
	indexLock    sync.RWMutex
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

	a.indexLock.Lock()
	defer a.indexLock.Unlock()

	var tokenizedLine []string
	a.indexBuilder = NewTrieIndex()
	a.corpus = make([]string, 0)
	scanner := bufio.NewScanner(file)
	buf := make([]byte, maxLineSize)
	scanner.Buffer(buf, maxLineSize)
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

	if a.index == nil {
		http.Error(w, "No corpus has been uploaded", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	query := r.URL.Query().Get("query")
	typeString := r.URL.Query().Get("type")
	operatorString := r.URL.Query().Get("operator")
	d := r.URL.Query().Get("distance")

	var dist int
	var err error
	if d == "" {
		dist = 0
	} else {
		dist, err = strconv.Atoi(d)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	var searchType SearchType
	switch typeString {
	case "exact":
		searchType = ExactSearch
	case "prefix":
		searchType = PrefixSearch
	case "fuzzy":
		searchType = FuzzySearch
	default:
		searchType = ExactSearch
	}

	var operator Operator
	switch operatorString {
	case "and":
		operator = And
	case "or":
		operator = Or
	default:
		operator = Or
	}

	a.indexLock.RLock()
	defer a.indexLock.RUnlock()

	searchResult := a.index.Search(query, searchType, operator, dist)
	matching_ids := a.index.Rank(searchResult.tokens, searchResult.DocIds())
	result := make([]searchResponse, 0)

	var response searchResponse
	for _, res := range matching_ids {
		response = searchResponse{Id: res.id, Score: math.Round(1000 * res.score), Text: a.corpus[res.id]}
		result = append(result, response)
	}

	err = json.NewEncoder(w).Encode(result)
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

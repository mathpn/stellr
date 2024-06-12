package main

import "testing"

type prefixTest struct {
	word   string
	prefix bool
	insert bool
}

func TestPatriciaTrieSearch(t *testing.T) {
	trie := NewPatriciaTrie()
	inserts := []string{
		"orange",
		"organism",
		"apple",
		"ape",
		"cat",
		"can",
		"foo",
		"the",
		"then",
		"bar",
		"organization",
		"organizations",
		"oranges",
		"organized",
		"organs",
		"horror",
		"ore",
		"oregon",
		"or",
	}
	var found bool
	for _, word := range inserts {
		found = trie.Search(word)
		if found {
			t.Errorf("word %s should not be found in trie", word)
		}
		trie.Insert(word, nil)

		found = trie.Search(word)
		if !found {
			t.Errorf("word %s should be found in trie", word)
		}
	}

	for _, word := range inserts {
		found = trie.Search(word)
		if !found {
			t.Errorf("word %s should be found in trie", word)
		}
		trie.Insert(word, nil)
	}
}

func TestPatriciaTriePrefix(t *testing.T) {
	trie := NewPatriciaTrie()
	tests := []prefixTest{
		{word: "ca", prefix: false, insert: false},
		{word: "c", prefix: false, insert: false},
		{word: "cat", prefix: false, insert: true},
		{word: "can", prefix: false, insert: true},
		{word: "ca", prefix: true, insert: false},
		{word: "the", prefix: false, insert: true},
		{word: "then", prefix: false, insert: true},
		{word: "the", prefix: true, insert: true},
		{word: "the", prefix: true, insert: true},
	}

	var isPrefix bool
	for _, prefixTest := range tests {
		isPrefix = trie.StartsWith(prefixTest.word)
		if isPrefix != prefixTest.prefix {
			t.Errorf(
				"trie prefix search failed for word %s. Expected %v got %v",
				prefixTest.word,
				prefixTest.prefix,
				isPrefix,
			)
		}
		if prefixTest.insert {
			trie.Insert(prefixTest.word, nil)
		}
	}
}

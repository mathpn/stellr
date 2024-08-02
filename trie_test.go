package main

import (
	"testing"

	"github.com/RoaringBitmap/roaring"
)

type prefixTest struct {
	set       *roaring.Bitmap
	prefixSet *roaring.Bitmap
	word      string
	prefix    bool
	insert    bool
}

type searchTest struct {
	word        string
	inTrie      bool
	set         *roaring.Bitmap
	expectedSet *roaring.Bitmap
}

func TestPatriciaTrieSearch(t *testing.T) {
	trie := NewPatriciaTrie()
	inserts := []searchTest{
		{"orange", false, roaring.BitmapOf(1), roaring.BitmapOf(1)},
		{"organism", false, roaring.BitmapOf(2), roaring.BitmapOf(2)},
		{"apple", false, roaring.BitmapOf(3), roaring.BitmapOf(3)},
		{"ape", false, roaring.BitmapOf(4), roaring.BitmapOf(4)},
		{"cat", false, roaring.BitmapOf(5), roaring.BitmapOf(5)},
		{"can", false, roaring.BitmapOf(6), roaring.BitmapOf(6)},
		{"foo", false, roaring.BitmapOf(7), roaring.BitmapOf(7)},
		{"the", false, roaring.BitmapOf(8), roaring.BitmapOf(8)},
		{"then", false, roaring.BitmapOf(9), roaring.BitmapOf(9)},
		{"bar", false, roaring.BitmapOf(10), roaring.BitmapOf(10)},
		{"organization", false, roaring.BitmapOf(11), roaring.BitmapOf(11)},
		{"organizations", false, roaring.BitmapOf(12), roaring.BitmapOf(12)},
		{"oranges", false, roaring.BitmapOf(13), roaring.BitmapOf(13)},
		{"organized", false, roaring.BitmapOf(14), roaring.BitmapOf(14)},
		{"organs", false, roaring.BitmapOf(15), roaring.BitmapOf(15)},
		{"horror", false, roaring.BitmapOf(16), roaring.BitmapOf(16)},
		{"ore", false, roaring.BitmapOf(17), roaring.BitmapOf(17)},
		{"oregon", false, roaring.BitmapOf(18), roaring.BitmapOf(18)},
		{"or", false, roaring.BitmapOf(19), roaring.BitmapOf(19)},
		{"or", true, roaring.BitmapOf(20), roaring.BitmapOf(19, 20)},
	}
	var found *IndexResult
	for _, insert := range inserts {
		found = trie.Search(insert.word)
		if found != nil && !insert.inTrie {
			t.Errorf("word %s should not be found in trie", insert.word)
		}
		trie.Insert(insert.word, insert.set)

		found = trie.Search(insert.word)
		if found == nil {
			t.Errorf("word %s should be found in trie", insert.word)
		}
		if found != nil && !insert.expectedSet.Equals(found.set) {
			t.Errorf("wrong bitset returned for word %s", insert.word)
		}
	}
}

func TestPatriciaTriePrefix(t *testing.T) {
	trie := NewPatriciaTrie()
	tests := []prefixTest{
		{
			word: "ca", prefix: false, insert: false,
			set: roaring.BitmapOf(1),
		},
		{
			word: "c", prefix: false, insert: false,
			set: roaring.BitmapOf(2),
		},
		{
			word: "cat", prefix: false, insert: false,
			set: roaring.BitmapOf(3),
		},
		{
			word: "can", prefix: false, insert: true,
			set: roaring.BitmapOf(4),
		},
		{
			word: "ca", prefix: true, insert: false,
			set: roaring.BitmapOf(5), prefixSet: roaring.BitmapOf(4),
		},
		{
			word: "the", prefix: false, insert: true,
			set: roaring.BitmapOf(6),
		},
		{
			word: "then", prefix: false, insert: true,
			set: roaring.BitmapOf(7),
		},
		{
			word: "the", prefix: true, insert: true,
			set: roaring.BitmapOf(8), prefixSet: roaring.BitmapOf(6, 7),
		},
		{
			word: "the", prefix: true, insert: true,
			set: roaring.BitmapOf(8), prefixSet: roaring.BitmapOf(6, 7, 8),
		},
	}

	var result *IndexResult
	for _, prefixTest := range tests {
		result = trie.StartsWith(prefixTest.word)
		if (result == nil || !prefixTest.prefix) && (result != nil || prefixTest.prefix) {
			t.Errorf(
				"trie prefix search failed for word %s. Expected %v got %v",
				prefixTest.word,
				prefixTest.prefix,
				result,
			)
		}

		if prefixTest.prefixSet != nil {
			if !prefixTest.prefixSet.Equals(result.set) {
				t.Errorf("wrong bitset returned for word %s | %v exp %v", prefixTest.word, result, prefixTest.prefixSet)
			}
		}

		if prefixTest.insert {
			trie.Insert(prefixTest.word, prefixTest.set)
		}
	}
}

type fuzzySearchTest struct {
	word        string
	distance    int
	inTrie      bool
	set         *roaring.Bitmap
	expectedSet *roaring.Bitmap
}

func TestPatriciaTrieFuzzySearch(t *testing.T) {
	trie := NewPatriciaTrie()
	inserts := []fuzzySearchTest{
		{"orange", 0, false, roaring.BitmapOf(1), roaring.BitmapOf(1)},
		{"orang", 1, true, roaring.BitmapOf(1), roaring.BitmapOf(1)},
		{"organism", 0, false, roaring.BitmapOf(2), roaring.BitmapOf(2)},
		{"oregon", 0, false, roaring.BitmapOf(18), roaring.BitmapOf(18)},
		{"ore", 3, true, roaring.BitmapOf(17), roaring.BitmapOf(1, 17, 18)},
		{"ore", 1, true, roaring.BitmapOf(17), roaring.BitmapOf(17)},
		{"ori", 0, false, roaring.BitmapOf(19), roaring.BitmapOf(19)},
	}
	var found *IndexResult
	for _, insert := range inserts {
		found = trie.Search(insert.word)
		if found != nil && !insert.inTrie {
			t.Errorf("word %s should not be found in trie", insert.word)
		}
		trie.Insert(insert.word, insert.set)

		found = trie.FuzzySearch(insert.word, insert.distance)
		if found == nil {
			t.Errorf("word %s should be found in trie", insert.word)
		}
		if found != nil && !insert.expectedSet.Equals(found.set) {
			t.Errorf("wrong bitset returned for word %s: %v vs %v", insert.word, found.set, insert.expectedSet)
		}
	}
}

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
	word string
	set  roaring.Bitmap
}

func TestPatriciaTrieSearch(t *testing.T) {
	trie := NewPatriciaTrie()
	inserts := []searchTest{
		{"orange", *roaring.BitmapOf(1)},
		{"organism", *roaring.BitmapOf(2)},
		{"apple", *roaring.BitmapOf(3)},
		{"ape", *roaring.BitmapOf(4)},
		{"cat", *roaring.BitmapOf(5)},
		{"can", *roaring.BitmapOf(6)},
		{"foo", *roaring.BitmapOf(7)},
		{"the", *roaring.BitmapOf(8)},
		{"then", *roaring.BitmapOf(9)},
		{"bar", *roaring.BitmapOf(10)},
		{"organization", *roaring.BitmapOf(11)},
		{"organizations", *roaring.BitmapOf(12)},
		{"oranges", *roaring.BitmapOf(13)},
		{"organized", *roaring.BitmapOf(14)},
		{"organs", *roaring.BitmapOf(15)},
		{"horror", *roaring.BitmapOf(16)},
		{"ore", *roaring.BitmapOf(17)},
		{"oregon", *roaring.BitmapOf(18)},
		{"or", *roaring.BitmapOf(19)},
	}
	var found *roaring.Bitmap
	for _, insert := range inserts {
		found = trie.Search(insert.word)
		if found != nil {
			t.Errorf("word %s should not be found in trie", insert.word)
		}
		trie.Insert(insert.word, &insert.set)

		found = trie.Search(insert.word)
		if found == nil {
			t.Errorf("word %s should be found in trie", insert.word)
		}
		if !insert.set.Equals(found) {
			t.Errorf("wrong bitset returned for word %s", insert.word)
		}
	}

	for _, insert := range inserts {
		found = trie.Search(insert.word)
		if found == nil {
			t.Errorf("word %s should be found in trie", insert.word)
		}
		trie.Insert(insert.word, &insert.set)

		if !insert.set.Equals(found) {
			t.Errorf("wrong bitset returned for word %s", insert.word)
		}
	}
}

// func TestPatriciaTriePrefix(t *testing.T) {
// 	trie := NewPatriciaTrie()
// 	tests := []prefixTest{
// 		{word: "ca", prefix: false, insert: false, set: roaring.BitmapOf(1)},
// 		{word: "c", prefix: false, insert: false, set: roaring.BitmapOf(2)},
// 		{word: "cat", prefix: false, insert: true, set: roaring.BitmapOf(3)},
// 		{word: "can", prefix: false, insert: true, set: roaring.BitmapOf(4)},
// 		{word: "ca", prefix: true, insert: false, set: roaring.BitmapOf(5), prefixSet: roaring.BitmapOf(4)},
// 		{word: "the", prefix: false, insert: true, set: roaring.BitmapOf(6)},
// 		{word: "then", prefix: false, insert: true, set: roaring.BitmapOf(7)},
// 		{word: "the", prefix: true, insert: true, set: roaring.BitmapOf(8), prefixSet: roaring.BitmapOf(7)},
// 		{word: "the", prefix: true, insert: true, set: roaring.BitmapOf(9), prefixSet: roaring.BitmapOf(7)},
// 	}
//
// 	var result *roaring.Bitmap
// 	for _, prefixTest := range tests {
// 		result = trie.StartsWith(prefixTest.word)
// 		if (result == nil || !prefixTest.prefix) && (result != nil || prefixTest.prefix) {
// 			t.Errorf(
// 				"trie prefix search failed for word %s. Expected %v got %v",
// 				prefixTest.word,
// 				prefixTest.prefix,
// 				result,
// 			)
// 		}
// 		if prefixTest.insert {
// 			trie.Insert(prefixTest.word, prefixTest.set)
// 		}
// 	}
// }

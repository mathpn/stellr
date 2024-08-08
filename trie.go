package main

import (
	"fmt"
	"strings"

	"github.com/RoaringBitmap/roaring"
)

type node struct {
	parent   *edge
	value    *roaring.Bitmap
	children []*node
}

type edge struct {
	id  int
	len int
}

type PatriciaTrie struct {
	root    *node
	strings []string
}

func NewPatriciaTrie() *PatriciaTrie {
	return &PatriciaTrie{root: &node{}, strings: make([]string, 0)}
}

func (n *node) isLeaf() bool {
	return len(n.children) == 0
}

func (t *PatriciaTrie) Print() {
	fmt.Println("-> TRIE:")
	fmt.Printf("%v\n", t.strings)
	node := t.root
	t.print(node, 0, make([]string, 0))
}

func (t *PatriciaTrie) print(currentNode *node, length int, path []string) {
	if currentNode == nil {
		return
	}

	if currentNode.parent != nil {
		l := length + currentNode.parent.len
		edgeLabel := t.strings[currentNode.parent.id][length:l]
		edgeLabel = strings.Replace(edgeLabel, string('\x00'), "$", 1)
		path = append(path, fmt.Sprintf("[%d] %s", currentNode.parent.id, edgeLabel))
		length += currentNode.parent.len
	}

	if currentNode.isLeaf() {
		path := strings.Join(path, " -> ")
		fmt.Printf("PATH: %s\n", path)
		return
	}

	for _, childNode := range currentNode.children {
		t.print(childNode, length, path)
	}
}

func (t *PatriciaTrie) findChild(n *node, key string, elementsFound int) *node {
	var l int
	for _, childNode := range n.children {
		l = elementsFound + childNode.parent.len
		edgeLabel := t.strings[childNode.parent.id][elementsFound:l]

		if strings.HasPrefix(key, edgeLabel) {
			return childNode
		}
	}
	return nil
}

func (t *PatriciaTrie) findPrefix(n *node, key string, elementsFound int) (*node, int) {
	var overlap, l int
	for _, childNode := range n.children {
		l = elementsFound + childNode.parent.len
		edgeLabel := t.strings[childNode.parent.id][elementsFound:l]

		for ; overlap < len(key); overlap++ {
			if key[overlap] != edgeLabel[overlap] {
				break
			}
		}

		if overlap != 0 {
			return childNode, overlap
		}
	}
	return n, 0
}

func (t *PatriciaTrie) search(key string) (*node, int, int) {
	currentNode := t.root
	elementsFound := 0
	lenKey := len(key)

	var overlap int
	var nextNode *node
	for currentNode != nil {
		if elementsFound == lenKey {
			break
		}

		if currentNode.children == nil {
			break
		}

		nextNode = nil
		nextNode = t.findChild(currentNode, key, elementsFound)
		if nextNode == nil {
			currentNode, overlap = t.findPrefix(currentNode, key, elementsFound)
			elementsFound += overlap
			return currentNode, elementsFound, overlap
		}
		key = key[nextNode.parent.len:]
		elementsFound += nextNode.parent.len
		currentNode = nextNode
	}

	return currentNode, elementsFound, 0
}

func (t *PatriciaTrie) fuzzySearch(node *node, key string, limit int, length int, matchedNodes []*node) []*node {
	partialStr := ""
	if node.parent != nil {
		length += node.parent.len
		partialStr = t.strings[node.parent.id][0:length]
	}
	l := min(len(key), length)
	k := key[0:l]

	distance := LevenshteinDistance(partialStr, k)
	if distance <= limit {
		for _, child := range node.children {
			matchedNodes = t.fuzzySearch(child, key, limit, length, matchedNodes)
		}
	}

	if node.isLeaf() {
		if l < len(key) {
			distance = LevenshteinDistance(partialStr, key)
		}
		if distance <= limit {
			matchedNodes = append(matchedNodes, node)
		}
	}
	return matchedNodes
}

func (t *PatriciaTrie) Insert(key string, set *roaring.Bitmap) {
	key += string('\x00')
	lenKey := len(key)

	currentNode, elementsFound, overlap := t.search(key)
	if currentNode == nil {
		currentNode = t.root
	}

	if elementsFound == lenKey {
		currentNode.value.Or(set)
		return
	}

	if elementsFound == 0 {
		t.insertRootChild(currentNode, key, set)
	} else {
		t.insertNode(currentNode, key, set, elementsFound, overlap)
	}
}

func (t *PatriciaTrie) insertRootChild(n *node, key string, set *roaring.Bitmap) {
	t.strings = append(t.strings, key)
	edge := &edge{id: len(t.strings) - 1, len: len(key)}
	childNode := &node{parent: edge, value: set}
	n.children = append(n.children, childNode)
}

func (t *PatriciaTrie) insertNode(n *node, key string, set *roaring.Bitmap, elementsFound int, overlap int) {
	idx := n.parent.id
	lenKey := len(key)

	if overlap != 0 {
		splitEdge := &edge{id: idx, len: n.parent.len - overlap}
		splitNode := &node{parent: splitEdge}
		splitNode.children = n.children
		splitNode.value = n.value
		n.children = []*node{splitNode}
		n.value = nil
		n.parent.len = overlap
	}

	t.strings = append(t.strings, key)
	idx = len(t.strings) - 1
	newEdge := &edge{id: idx, len: lenKey - elementsFound}
	newNode := &node{parent: newEdge, value: set}
	n.children = append(n.children, newNode)
}

func (t *PatriciaTrie) Search(key string) *IndexResult {
	key += string('\x00')
	n, elementsFound, _ := t.search(key)
	if elementsFound == len(key) {
		label := t.strings[n.parent.id]
		label = label[0 : len(label)-1]
		return &IndexResult{set: n.value, tokens: []string{label}}
	}
	return nil
}

func (t *PatriciaTrie) FuzzySearch(key string, limit int) *IndexResult {
	key += string('\x00')
	nodes := t.fuzzySearch(t.root, key, limit, 0, make([]*node, 0))
	res := &IndexResult{set: roaring.New(), tokens: make([]string, 0)}

	var r *IndexResult
	for _, n := range nodes {
		label := t.strings[n.parent.id]
		label = label[0 : len(label)-1]
		r = &IndexResult{set: n.value, tokens: []string{label}}
		res.CombineOr(r)
	}
	return res
}

type IndexResult struct {
	set    *roaring.Bitmap
	tokens []string
}

func (r *IndexResult) CombineOr(res *IndexResult) {
	if r.set == nil {
		r.set = res.set.Clone()
	} else {
		r.set.Or(res.set)
	}
	r.tokens = append(r.tokens, res.tokens...)
}

func (r *IndexResult) CombineAnd(res *IndexResult) {
	if r.set == nil {
		r.set = res.set.Clone()
	} else {
		r.set.And(res.set)
	}
	r.tokens = append(r.tokens, res.tokens...)
}

func (t *PatriciaTrie) mergeChildren(n *node, result *IndexResult) *IndexResult {
	if n.isLeaf() {
		label := t.strings[n.parent.id]
		label = label[0 : len(label)-1]
		result.tokens = append(result.tokens, label)
		result.set.Or(n.value)
		return result
	}

	for _, child := range n.children {
		result = t.mergeChildren(child, result)
	}
	return result
}

func (t *PatriciaTrie) StartsWith(key string) *IndexResult {
	n, elementsFound, _ := t.search(key)
	if elementsFound == len(key) {
		return t.mergeChildren(n, &IndexResult{set: roaring.New(), tokens: make([]string, 0)})
	}
	return nil
}

type tokenSet struct {
	set   *roaring.Bitmap
	token string
}

func (t *PatriciaTrie) Traversal() []tokenSet {
	path := []tokenSet{}
	processNode := func(node *node) {
		if node.value == nil {
			return
		}
		token := t.strings[node.parent.id]
		token = token[:len(token)-1]
		path = append(path, tokenSet{set: node.value, token: token})
	}
	walkIn(t.root, processNode)
	return path
}

func walkIn(curr *node, processNode func(*node)) {
	if curr == nil {
		return
	}
	processNode(curr)
	for _, n := range curr.children {
		walkIn(n, processNode)
	}
}

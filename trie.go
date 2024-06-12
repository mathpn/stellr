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

		for ; overlap < len(key)-elementsFound; overlap++ {
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

func (t *PatriciaTrie) Insert(key string, set *roaring.Bitmap) {
	key += string('\x00')
	lenKey := len(key)

	currentNode, elementsFound, overlap := t.search(key)
	if currentNode == nil {
		currentNode = t.root
	}

	if elementsFound == lenKey {
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

func (t *PatriciaTrie) Search(key string) *roaring.Bitmap {
	key += string('\x00')
	n, elementsFound, _ := t.search(key)
	if elementsFound == len(key) {
		return n.value
	}
	return nil
}

// FIXME traverse children
func (t *PatriciaTrie) StartsWith(key string) *roaring.Bitmap {
	n, elementsFound, _ := t.search(key)
	if elementsFound == len(key) {
		return n.value
	}
	return nil
}

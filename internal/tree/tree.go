package tree

import (
	"sort"
	"strings"
)

type node struct {
	name     string
	children []*node
}

func Build(paths []string, rootName string) string {
	root := &node{name: rootName}

	for _, path := range paths {
		segments := splitPath(path)
		current := root
		for _, seg := range segments {
			child := findChild(current, seg)
			if child == nil {
				child = &node{name: seg}
				current.children = append(current.children, child)
			}
			current = child
		}
	}

	sortTree(root)

	var lines []string
	if root.name != "" {
		lines = append(lines, root.name)
	}
	renderNode(&lines, root, "")
	return strings.Join(lines, "\n")
}

func splitPath(path string) []string {
	path = strings.ReplaceAll(path, "\\", "/")
	var parts []string
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func findChild(n *node, name string) *node {
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

func sortTree(n *node) {
	sort.Slice(n.children, func(i, j int) bool {
		return n.children[i].name < n.children[j].name
	})
	for _, c := range n.children {
		sortTree(c)
	}
}

func renderNode(lines *[]string, n *node, indent string) {
	for i, child := range n.children {
		isLast := i == len(n.children)-1
		prefix := "├── "
		if isLast {
			prefix = "└── "
		}
		*lines = append(*lines, indent+prefix+child.name)
		newIndent := indent + "│   "
		if isLast {
			newIndent = indent + "    "
		}
		renderNode(lines, child, newIndent)
	}
}

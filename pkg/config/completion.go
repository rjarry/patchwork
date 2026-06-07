// Patchwork - automated patch tracking system
// Copyright (C) 2026 Robin Jarry <robin@jarry.cc>
//
// SPDX-License-Identifier: GPL-2.0-or-later

package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/alecthomas/kong"
)

func bashComplete(app *kong.Kong) bool {
	compLine := os.Getenv("COMP_LINE")
	if compLine == "" {
		return false
	}
	compPoint, err := strconv.Atoi(os.Getenv("COMP_POINT"))
	if err != nil || compPoint > len(compLine) {
		compPoint = len(compLine)
	}
	line := compLine[:compPoint]

	words := strings.Fields(line)
	// remove the program name
	if len(words) > 0 {
		words = words[1:]
	}
	// if the line ends with a space, we're completing a new word
	trailing := strings.HasSuffix(line, " ")

	node := app.Model.Node
	prefix := ""

	seen := make(map[string]bool)
	for _, w := range words {
		seen[w] = true
	}

	// walk into subcommands, skipping flags and their values
	for len(words) > 0 {
		w := words[0]
		if strings.HasPrefix(w, "-") {
			// skip flag; also skip its value if it takes one
			if f := findFlag(node, w); f != nil && !f.IsBool() && !f.IsCounter() {
				words = words[1:]
			}
			words = words[1:]
			continue
		}
		child := findChild(node, w)
		if child == nil {
			break
		}
		node = child
		words = words[1:]
	}

	if len(words) > 0 && !trailing {
		prefix = words[len(words)-1]
	}

	maxWidth := 0
	type candidate struct {
		value, help string
	}
	var candidates []candidate

	add := func(value, help string) {
		if !strings.HasPrefix(value, prefix) {
			return
		}
		if seen[value] {
			return
		}
		candidates = append(candidates, candidate{value, help})
		if len(value) > maxWidth {
			maxWidth = len(value)
		}
	}

	// complete subcommands
	for _, child := range node.Children {
		if child.Hidden {
			continue
		}
		add(child.Name, child.Help)
	}

	// complete flags from current node and all ancestors
	for n := node; n != nil; n = n.Parent {
		for _, flag := range n.Flags {
			if flag.Hidden || flag.Name == "help" {
				continue
			}
			long := "--" + flag.Name
			short := ""
			if flag.Short != 0 {
				short = "-" + string(flag.Short)
			}
			if seen[long] || seen[short] {
				continue
			}
			add(long, flag.Help)
		}
	}

	for _, c := range candidates {
		if len(candidates) > 1 && c.help != "" {
			fmt.Printf("%-*s    (%s)\n", maxWidth, c.value, c.help)
		} else {
			fmt.Println(c.value)
		}
	}

	return true
}

func findFlag(node *kong.Node, name string) *kong.Flag {
	name = strings.TrimLeft(name, "-")
	// also handle --flag=value
	name, _, _ = strings.Cut(name, "=")
	for n := node; n != nil; n = n.Parent {
		for _, f := range n.Flags {
			if f.Name == name || (f.Short != 0 && string(f.Short) == name) {
				return f
			}
		}
	}
	return nil
}

func findChild(node *kong.Node, name string) *kong.Node {
	for _, child := range node.Children {
		if child.Name == name || slices.Contains(child.Aliases, name) {
			return child
		}
	}
	return nil
}

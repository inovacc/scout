// Package okf implements the Open Knowledge Format (OKF) v0.1 — a lightweight
// convention for storing structured knowledge as a directory tree of Markdown
// files with YAML frontmatter.
//
// A Bundle is a directory of Concept files. Each Concept is one Markdown file
// whose ID is the bundle-relative path without the ".md" extension (forward
// slashes). Relationships between concepts are ordinary Markdown links in the
// body text.
//
// Usage:
//
//	b := &okf.Bundle{Concepts: []okf.Concept{
//	    {ID: "intro", Type: "Page", Title: "Introduction", Body: "Hello."},
//	}}
//	_ = b.Write("/path/to/dir")
//
//	b2, _ := okf.Read("/path/to/dir")
//	_ = b2.Validate()
package okf

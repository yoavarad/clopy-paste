// Package web embeds the HTML pages generated from ydk:page manifests.
package web

import "embed"

// Pages holds one <name>.html file per ydk:page.
//
//go:embed *.html
var Pages embed.FS

// SplitJS is web/split.js (ADR-019), inlined into main.html by the index handler.
//
//go:embed split.js
var SplitJS string

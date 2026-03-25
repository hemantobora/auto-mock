package collections

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// parseOpenCollectionFile parses a Bruno-exported bundled OpenCollection YAML file
// (identified by "bundled: true" at the top level). All requests are nested under
// the top-level "items:" array, each starting with "  - info:".
//
// Strategy: we split the bundled file into per-item YAML chunks by detecting
// "  - info:" boundaries, de-indent each chunk by 4 spaces so it matches the
// single-file format, then re-use the existing parseOCYAML parser on each chunk.
func (cp *CollectionProcessor) parseOpenCollectionFile(filePath string) ([]APIRequest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenCollection file: %w", err)
	}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	// ── Split into per-item chunks ────────────────────────────────────────────
	// Each item begins with "  - info:" (2-space indent + list marker).
	// We convert that line to "info:" and strip 4 leading spaces from every
	// subsequent line so the chunk looks like a standalone per-request YAML file.
	var chunks [][]string
	var cur []string

	for _, line := range lines {
		if strings.HasPrefix(line, "  - info:") {
			if cur != nil {
				chunks = append(chunks, cur)
			}
			cur = []string{"info:"}
			continue
		}
		if cur == nil {
			continue // haven't entered items section yet
		}
		// A line starting at column 0 exits the items section (e.g. "bundled:", "extensions:")
		if len(line) > 0 && line[0] != ' ' {
			chunks = append(chunks, cur)
			cur = nil
			continue
		}
		if len(line) == 0 {
			cur = append(cur, "")
			continue
		}
		// Strip 4 leading spaces (outer items array offset)
		if len(line) >= 4 {
			cur = append(cur, line[4:])
		} else {
			cur = append(cur, strings.TrimLeft(line, " "))
		}
	}
	if cur != nil {
		chunks = append(chunks, cur)
	}

	// ── Parse + sort by seq ───────────────────────────────────────────────────
	type seqDoc struct {
		doc *ocDoc
		seq int
	}
	var parsed []seqDoc
	for _, chunk := range chunks {
		doc := parseOCYAML([]byte(strings.Join(chunk, "\n")))
		if doc.infoType == "http" {
			parsed = append(parsed, seqDoc{doc: doc, seq: doc.infoSeq})
		}
	}

	sort.Slice(parsed, func(i, j int) bool {
		return parsed[i].seq < parsed[j].seq
	})

	// ── Convert to APIRequest + surface script notes ──────────────────────────
	var apis []APIRequest
	for idx, pd := range parsed {
		api := pd.doc.toAPIRequest(idx + 1)
		if api.Method == "" {
			continue
		}
		// Show script notes so users understand flow dependencies
		// (scripts run client-side in Bruno; we capture them as informational metadata)
		if pd.doc.preScript != "" {
			fmt.Printf("  ↳ [%s] pre-request:    %s\n", api.Name, pd.doc.preScript)
		}
		if pd.doc.postScript != "" {
			fmt.Printf("  ↳ [%s] after-response: %s\n", api.Name, pd.doc.postScript)
		}
		apis = append(apis, api)
	}

	if len(apis) == 0 {
		return nil, fmt.Errorf("no HTTP requests found in OpenCollection file")
	}

	fmt.Printf("✅ Parsed %d OpenCollection requests\n", len(apis))
	return apis, nil
}

// ── YAML document model ───────────────────────────────────────────────────────

type ocDoc struct {
	infoName   string
	infoType   string
	infoSeq    int
	method     string
	url        string
	headers    map[string]string // name → value (enabled items only)
	query      map[string]string // name → value (enabled items only)
	bodyType   string
	bodyData   string
	preScript  string
	postScript string
}

// toAPIRequest converts the parsed YAML doc into the common APIRequest type.
func (d *ocDoc) toAPIRequest(idx int) APIRequest {
	name := d.infoName
	if name == "" {
		name = fmt.Sprintf("Request %d", idx)
	}
	return APIRequest{
		ID:          fmt.Sprintf("oc_%d", idx),
		Name:        name,
		Method:      d.method,
		URL:         d.url,
		Headers:     d.headers,
		QueryParams: d.query,
		Body:        d.bodyData,
		PreScript:   d.preScript,
		PostScript:  d.postScript,
		Variables:   make(map[string]string),
	}
}

// ── Minimal YAML parser ───────────────────────────────────────────────────────
//
// Handles the subset of YAML used by OpenCollection request files.
// Uses an indentation-aware state machine — no external dependency needed.
func parseOCYAML(data []byte) *ocDoc {
	doc := &ocDoc{
		headers: make(map[string]string),
		query:   make(map[string]string),
	}

	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	// sectionLevel[depth] = current section key at that indent depth
	sectionLevel := make(map[int]string)
	sec := func(depth int) string { return sectionLevel[depth] }

	// List item accumulators
	liName    := ""   // most recent "name:" value in a list item
	liEnabled := true // most recent "enabled:" value (default true)

	// Block scalar (literal |-) collection state
	var (
		blockDest   *string
		blockLines  []string
		blockIndent int
		inBlock     bool
	)

	flushBlock := func() {
		if blockDest != nil && len(blockLines) > 0 {
			*blockDest = strings.TrimRight(strings.Join(blockLines, "\n"), "\n")
		}
		blockLines = nil
		blockDest = nil
		inBlock = false
		blockIndent = 0
	}

	indentOf := func(s string) int {
		return len(s) - len(strings.TrimLeft(s, " \t"))
	}

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)

		// ── Block scalar collection ───────────────────────────────────────────
		if inBlock {
			ind := indentOf(rawLine)
			if trimmed == "" || ind >= blockIndent {
				prefix := strings.Repeat(" ", blockIndent)
				blockLines = append(blockLines, strings.TrimPrefix(rawLine, prefix))
				continue
			}
			flushBlock()
			// Fall through to process this line normally
		}

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		ind := indentOf(rawLine)
		depth := ind / 2

		// Evict deeper sections when returning to a shallower level
		for k := range sectionLevel {
			if k >= depth {
				delete(sectionLevel, k)
			}
		}

		// Strip list item marker — each "- " resets the current list item
		isList := strings.HasPrefix(trimmed, "- ")
		if isList {
			trimmed = strings.TrimPrefix(trimmed, "- ")
			liName = ""
			liEnabled = true
		}

		// Parse key[:] [value]
		var key, val string
		isSection := false
		isBlock := false

		if ci := strings.Index(trimmed, ": "); ci >= 0 {
			key = trimmed[:ci]
			val = strings.TrimSpace(trimmed[ci+2:])
			// Strip surrounding quotes
			if len(val) >= 2 {
				if (val[0] == '"' && val[len(val)-1] == '"') ||
					(val[0] == '\'' && val[len(val)-1] == '\'') {
					val = val[1 : len(val)-1]
				}
			}
			isBlock = val == "|" || val == "|-" || val == "|+" || val == ">" || val == ">-"
			if isBlock {
				val = ""
			}
		} else if strings.HasSuffix(trimmed, ":") {
			key = strings.TrimSuffix(trimmed, ":")
			isSection = true
		} else {
			continue // unrecognised line
		}

		// Treat empty-value keys as section headers
		if !isSection && !isBlock && val == "" {
			isSection = true
		}
		if isSection {
			sectionLevel[depth] = key
			continue
		}

		s0, s1, s2 := sec(0), sec(1), sec(2)

		switch {
		// ── info ──────────────────────────────────────────────────────────────
		case s0 == "info" && !isList && key == "name":
			doc.infoName = val
		case s0 == "info" && !isList && key == "type":
			doc.infoType = val
		case s0 == "info" && !isList && key == "seq":
			doc.infoSeq, _ = strconv.Atoi(val)

		// ── http top-level fields ─────────────────────────────────────────────
		case s0 == "http" && s1 == "" && key == "method":
			doc.method = strings.ToUpper(val)
		case s0 == "http" && s1 == "" && key == "url":
			doc.url = val

		// ── http.headers list items ───────────────────────────────────────────
		case s0 == "http" && s1 == "headers" && key == "name":
			liName = val
		case s0 == "http" && s1 == "headers" && key == "value" && liName != "":
			if liEnabled {
				doc.headers[liName] = val
			}
		case s0 == "http" && s1 == "headers" && key == "enabled":
			liEnabled = val != "false"

		// ── http.params / http.params.query list items ────────────────────────
		case s0 == "http" && (s1 == "params" || s2 == "query") && key == "name":
			liName = val
		case s0 == "http" && (s1 == "params" || s2 == "query") && key == "value" && liName != "":
			if liEnabled {
				doc.query[liName] = val
			}
		case s0 == "http" && (s1 == "params" || s2 == "query") && key == "enabled":
			liEnabled = val != "false"

		// ── http.body ─────────────────────────────────────────────────────────
		case s0 == "http" && s1 == "body" && key == "type":
			doc.bodyType = val
		case s0 == "http" && s1 == "body" && key == "data":
			if isBlock {
				inBlock = true
				blockIndent = ind + 2
				blockDest = &doc.bodyData
			} else if val != "" {
				doc.bodyData = val
			}
			// else: val == "" means a sub-list follows (e.g. form-urlencoded fields);
			// "data:" becomes a section header so sectionLevel[depth] = "data" — handled below.

		// ── http.body.data list (form-urlencoded / multipart) ─────────────────
		// Each list item is:  - name: key  /  value: val  /  enabled: true|false
		case s0 == "http" && s1 == "body" && s2 == "data" && key == "name":
			liName = val
		case s0 == "http" && s1 == "body" && s2 == "data" && key == "value" && liName != "":
			if liEnabled {
				if doc.bodyData != "" {
					doc.bodyData += "&"
				}
				doc.bodyData += liName + "=" + val
			}
		case s0 == "http" && s1 == "body" && s2 == "data" && key == "enabled":
			liEnabled = val != "false"

		// ── runtime.scripts list items ────────────────────────────────────────
		case s0 == "runtime" && s1 == "scripts" && key == "type":
			liName = val // repurpose: tracks which script type we're in
		case s0 == "runtime" && s1 == "scripts" && key == "code":
			var dest *string
			switch liName {
			case "pre-request":
				dest = &doc.preScript
			case "tests", "post-response", "after-response":
				dest = &doc.postScript
			}
			if isBlock {
				inBlock = true
				blockIndent = ind + 2
				blockDest = dest
			} else if dest != nil {
				*dest = val
			}
		}
	}

	// Flush any pending block scalar at EOF
	if inBlock {
		flushBlock()
	}

	return doc
}

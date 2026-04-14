package collections

import (
	"encoding/json"
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
		if doc.infoType == "http" || doc.infoType == "graphql" {
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
		return nil, fmt.Errorf("no HTTP or GraphQL requests found in OpenCollection file")
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

	// GraphQL-specific (populated when infoType == "graphql")
	gqlQuery string // raw query string (may have real newlines from block scalar or multi-line YAML)
	gqlVars  string // raw variables string (may contain YAML escape sequences if from quoted value)
}

// toAPIRequest converts the parsed YAML doc into the common APIRequest type.
func (d *ocDoc) toAPIRequest(idx int) APIRequest {
	name := d.infoName
	if name == "" {
		name = fmt.Sprintf("Request %d", idx)
	}

	// Unescape YAML double-quoted string sequences (e.g. \n, \t, \") from the
	// HTTP body. This is a no-op when the body came from a block scalar (|-),
	// since block scalars contain real bytes rather than escape sequences.
	body := unescapeYAMLString(d.bodyData)
	if d.infoType == "graphql" {
		body = buildGraphQLBody(d.gqlQuery, d.gqlVars)
	}

	return APIRequest{
		ID:          fmt.Sprintf("oc_%d", idx),
		Name:        name,
		Method:      d.method,
		URL:         d.url,
		Headers:     d.headers,
		QueryParams: d.query,
		Body:        body,
		PreScript:   d.preScript,
		PostScript:  d.postScript,
		Variables:   make(map[string]string),
	}
}

// buildGraphQLBody assembles the JSON body {"query": ..., "variables": ...} from
// the raw query string and raw variables string captured by the YAML parser.
// It handles:
//   - YAML double-quoted escape sequences in variables (\n, \t, \", \\)
//   - JS-style // line comments inside the variables block (Bruno habit)
func buildGraphQLBody(query, variables string) string {
	body := map[string]any{
		"query": strings.TrimSpace(query),
	}

	vars := strings.TrimSpace(unescapeYAMLString(variables))
	vars = stripLineComments(vars)
	vars = strings.TrimSpace(vars)

	if vars != "" {
		var varsObj any
		if err := json.Unmarshal([]byte(vars), &varsObj); err == nil {
			body["variables"] = varsObj
		} else {
			// Keep as raw string if it can't be parsed as JSON
			body["variables"] = vars
		}
	}

	b, err := json.Marshal(body)
	if err != nil {
		return `{"query":` + string(mustMarshalStr(query)) + `}`
	}
	return string(b)
}

// unescapeYAMLString converts YAML double-quoted string escape sequences to their
// actual characters. Safe to call on block-scalar content (no-op).
func unescapeYAMLString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case 't':
				b.WriteByte('\t')
				i++
				continue
			case '"':
				b.WriteByte('"')
				i++
				continue
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// stripLineComments removes lines whose trimmed content starts with // (JS-style comment).
func stripLineComments(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "//") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func mustMarshalStr(s string) []byte {
	b, _ := json.Marshal(s)
	return b
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

	// Multi-line YAML double-quoted string collection state.
	// Bruno sometimes emits a GraphQL query as a double-quoted scalar that spans
	// many physical lines in the file.  Standard YAML libraries handle this, but
	// our hand-rolled parser needs an explicit accumulation mode.
	var (
		mlQDest  *string
		mlQLines []string
		inMLQStr bool
	)

	flushMLQ := func() {
		if mlQDest != nil {
			*mlQDest = strings.Join(mlQLines, "\n")
		}
		mlQLines = nil
		mlQDest = nil
		inMLQStr = false
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

		// ── Multi-line double-quoted string collection ────────────────────────
		// A value like: query: "query Foo { ... }\n..." where the closing " is
		// on a later line.  We accumulate each trimmed line; the closing " is
		// detected as the last character of a line that does NOT end with \".
		if inMLQStr {
			// Empty lines are valid content inside a GQL query
			if trimmed == "" {
				mlQLines = append(mlQLines, "")
				continue
			}
			if strings.HasSuffix(trimmed, `"`) && !strings.HasSuffix(trimmed, `\"`) {
				// Closing quote — add content up to (not including) the quote
				mlQLines = append(mlQLines, trimmed[:len(trimmed)-1])
				flushMLQ()
			} else {
				mlQLines = append(mlQLines, trimmed)
			}
			continue
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

		// ── graphql top-level fields ──────────────────────────────────────────────
		case s0 == "graphql" && s1 == "" && key == "method":
			doc.method = strings.ToUpper(val)
		case s0 == "graphql" && s1 == "" && key == "url":
			doc.url = val

		// ── graphql.headers list items ────────────────────────────────────────────
		case s0 == "graphql" && s1 == "headers" && key == "name":
			liName = val
		case s0 == "graphql" && s1 == "headers" && key == "value" && liName != "":
			if liEnabled {
				doc.headers[liName] = val
			}
		case s0 == "graphql" && s1 == "headers" && key == "enabled":
			liEnabled = val != "false"

		// ── graphql.body.query ────────────────────────────────────────────────────
		// Supports three forms:
		//   query: |-           → block scalar
		//   query: "short..."   → single-line double-quoted string (already unquoted above)
		//   query: "first line  → multi-line double-quoted string (closing " on a later line)
		case s0 == "graphql" && s1 == "body" && key == "query":
			if isBlock {
				inBlock = true
				blockIndent = ind + 2
				blockDest = &doc.gqlQuery
			} else if val != "" {
				// val still carries the opening " when the closing " is on a later line
				if val[0] == '"' && val[len(val)-1] != '"' {
					inMLQStr = true
					mlQDest = &doc.gqlQuery
					mlQLines = []string{val[1:]} // strip opening quote; content starts here
				} else {
					doc.gqlQuery = val
				}
			}

		// ── graphql.body.variables ────────────────────────────────────────────────
		// Variables are a JSON object stored as:
		//   variables: "{\n\t...}"   → single-line quoted (needs YAML unescape later)
		//   variables: |-\n  {...}   → block scalar (raw JSON, may have // comments)
		// Both forms stored raw; buildGraphQLBody handles unescaping & comment stripping.
		case s0 == "graphql" && s1 == "body" && key == "variables":
			if isBlock {
				inBlock = true
				blockIndent = ind + 2
				blockDest = &doc.gqlVars
			} else if val != "" {
				doc.gqlVars = val
			}

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

	// Flush any pending block scalar or multi-line string at EOF
	if inBlock {
		flushBlock()
	}
	if inMLQStr {
		flushMLQ()
	}

	return doc
}

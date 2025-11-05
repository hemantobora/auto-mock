package builders

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/hemantobora/auto-mock/internal/models"
)

// --- enums/aliases (adjust to your actual types) ---
type CompressionAlgo string

const (
	AlgoIdentity CompressionAlgo = "identity"
	AlgoGzip     CompressionAlgo = "gzip"
	AlgoDeflate  CompressionAlgo = "deflate"
)

type CompressionMode int

const (
	CompressionHeaderOnly CompressionMode = iota
	CompressionPreCompress
)

func setHeaderKV(h map[string][]string, k string, v []string) {
	h[k] = v
}

func mergeVary(h map[string][]string, token string) {
	cur := strings.TrimSpace(strings.Join(h["Vary"], ","))
	if cur == "" {
		h["Vary"] = []string{token}
		return
	}
	// avoid duplicates (case-insensitive)
	parts := strings.Split(cur, ",")
	for _, p := range parts {
		if strings.EqualFold(strings.TrimSpace(p), token) {
			h["Vary"] = []string{cur}
			return
		}
	}
	h["Vary"] = []string{cur + ", " + token}
}

// Minimal JSON-or-text content-type inference.
// (If you already have a better inferContentType, keep that.)
func inferContentType(body any) string {
	switch v := body.(type) {
	case string:
		s := strings.TrimSpace(v)
		if json.Valid([]byte(s)) {
			return "application/json"
		}
		return "text/plain; charset=utf-8"
	case []byte:
		b := bytes.TrimSpace(v)
		if json.Valid(b) {
			return "application/json"
		}
		return "application/octet-stream"
	case map[string]any, []any:
		return "application/json"
	default:
		return "application/json"
	}
}

// Compressors
func gzipBytes(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(in); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func deflateBytes(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(in); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BodyToBytes — use the robust version you built earlier.
// Here’s a compact variant that handles strings/maps/[]byte reasonably.
// Replace with your full-featured version if you have it.
func bodyToBytes(body any, explicitCT string) ([]byte, string, error) {
	ct := explicitCT
	if ct == "" {
		ct = inferContentType(body)
	}
	switch v := body.(type) {
	case nil:
		return []byte{}, ct, nil
	case []byte:
		return v, ct, nil
	case string:
		return []byte(v), ct, nil
	case map[string]any:
		// If already a MockServer wrapper → materialize correctly
		if tRaw, ok := v["type"]; ok {
			if t, _ := tRaw.(string); t != "" {
				switch strings.ToUpper(t) {
				case "BINARY":
					if b64, _ := v["base64Bytes"].(string); b64 != "" {
						raw, err := base64.StdEncoding.DecodeString(b64)
						return raw, "application/octet-stream", err
					}
				case "STRING":
					if s, _ := v["string"].(string); s != "" {
						return []byte(s), inferContentType(s), nil
					}
				case "JSON":
					if j, ok := v["json"]; ok {
						b, err := json.Marshal(j)
						return b, "application/json", err
					}
				}
			}
		}
		b, err := json.Marshal(v)
		return b, "application/json", err
	case []any:
		b, err := json.Marshal(v)
		return b, "application/json", err
	default:
		b, err := json.Marshal(v)
		return b, "application/json", err
	}
}

// --- Your feature function, fixed ---

// --- small helpers for []NameValues headers/cookies --------------------------

func getHeader(headers []models.NameValues, name string) ([]string, bool) {
	if i := headerIndex(headers, name); i >= 0 {
		return headers[i].Values, true
	}
	return nil, false
}

func deleteHeader(headers *[]models.NameValues, name string) {
	if i := headerIndex(*headers, name); i >= 0 {
		h := *headers
		*headers = append(h[:i], h[i+1:]...)
	}
}

func headerHasValue(values []string, needle string) bool {
	for _, v := range values {
		if strings.EqualFold(strings.TrimSpace(v), needle) {
			return true
		}
	}
	return false
}

// mergeVaryNV ensures "Accept-Encoding" is present in Vary (case-insensitive).
func mergeVaryNV(headers *[]models.NameValues, token string) {
	vals, ok := getHeader(*headers, "Vary")
	if !ok || len(vals) == 0 {
		SetNameValues(headers, "Vary", []string{token})
		return
	}
	// Vary can be a single comma-separated item; normalize into a set
	var parts []string
	for _, v := range vals {
		for _, p := range strings.Split(v, ",") {
			if s := strings.TrimSpace(p); s != "" {
				parts = append(parts, s)
			}
		}
	}
	if !headerHasValue(parts, token) {
		parts = append(parts, token)
	}
	SetNameValues(headers, "Vary", []string{strings.Join(parts, ", ")})
}

// --- your function rewritten for []NameValues --------------------------------

// applyCompression lets the user choose compression and updates Headers ([]NameValues)
func applyCompression() FeatureFunc {
	return func(exp *MockExpectation) error {
		fmt.Println("\n🗜️  Response Compression Configuration")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

		ensureNameValues(exp) // makes slices non-nil

		// 1) pick algorithm
		var algoStr string
		if err := survey.AskOne(&survey.Select{
			Message: "Compression algorithm:",
			Options: []string{"identity", "gzip", "deflate"},
			Default: "gzip",
		}, &algoStr, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		algo := CompressionAlgo(strings.Split(algoStr, " ")[0])

		// 2) pick mode
		var modeStr string
		if err := survey.AskOne(&survey.Select{
			Message: "Mode:",
			Options: []string{
				"headers-only  — set Content-Encoding/Vary, do NOT alter body",
				"pre-compress  — actually compress body and serve binary payload",
			},
			Default: "headers-only  — set Content-Encoding/Vary, do NOT alter body",
		}, &modeStr, survey.WithValidator(survey.Required)); err != nil {
			return err
		}
		mode := CompressionHeaderOnly
		if strings.HasPrefix(modeStr, "pre-compress") {
			mode = CompressionPreCompress
		}

		switch mode {

		case CompressionHeaderOnly:
			if algo == AlgoIdentity {
				deleteHeader(&exp.HttpResponse.Headers, "Content-Encoding")
			} else {
				SetNameValues(&exp.HttpResponse.Headers, "Content-Encoding", []string{string(algo)})
				mergeVaryNV(&exp.HttpResponse.Headers, "Accept-Encoding")
			}
			// Add Content-Type if missing
			if _, ok := getHeader(exp.HttpResponse.Headers, "Content-Type"); !ok {
				if ct := inferContentType(exp.HttpResponse.Body); ct != "" {
					SetNameValues(&exp.HttpResponse.Headers, "Content-Type", []string{ct})
				}
			}
			return nil

		case CompressionPreCompress:
			if algo == AlgoIdentity {
				deleteHeader(&exp.HttpResponse.Headers, "Content-Encoding")
				return nil
			}

			// Convert current body to raw bytes
			raw, ct, err := bodyToBytes(exp.HttpResponse.Body, "")
			if err != nil {
				return fmt.Errorf("cannot read body for compression: %w", err)
			}

			// Compress
			var out []byte
			switch algo {
			case AlgoGzip:
				out, err = gzipBytes(raw)
			case AlgoDeflate:
				out, err = deflateBytes(raw)
			default:
				return fmt.Errorf("unsupported pre-compress algorithm: %s", algo)
			}
			if err != nil {
				return fmt.Errorf("compression failed: %w", err)
			}

			// Set headers to reflect compressed entity
			SetNameValues(&exp.HttpResponse.Headers, "Content-Encoding", []string{string(algo)})
			mergeVaryNV(&exp.HttpResponse.Headers, "Accept-Encoding")
			if ct != "" {
				SetNameValues(&exp.HttpResponse.Headers, "Content-Type", []string{ct})
			}
			SetNameValues(&exp.HttpResponse.Headers, "Content-Length", []string{fmt.Sprintf("%d", len(out))})

			// Update ETag if present (entity bytes changed)
			if etVals, ok := getHeader(exp.HttpResponse.Headers, "ETag"); ok && len(etVals) > 0 && strings.TrimSpace(etVals[0]) != "" {
				sum := sha1.Sum(out)
				SetNameValues(&exp.HttpResponse.Headers, "ETag", []string{`"` + hex.EncodeToString(sum[:]) + `"`})
			}

			// Store compressed body using MockServer BINARY wrapper
			exp.HttpResponse.Body = map[string]any{
				"type":        "BINARY",
				"base64Bytes": base64.StdEncoding.EncodeToString(out),
			}
			return nil

		default:
			return fmt.Errorf("unknown compression mode")
		}
	}
}

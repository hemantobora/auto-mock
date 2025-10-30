package mcp

// ErrMissingKey is returned by providers when their API key env var is absent.
type ErrMissingKey string

func (e ErrMissingKey) Error() string {
	return "missing API key: " + string(e)
}

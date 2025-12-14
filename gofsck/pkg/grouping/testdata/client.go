package model

// This should be allowed in http_client.go due to the allowlist.
type HTTPClient struct {
	baseURL string
}

func (c *HTTPClient) Request(path string) error {
	return nil
}

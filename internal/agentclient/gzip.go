package client

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"

	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func (c *Client) resultRequestBody(req protocol.AgentResultRequest) (any, error) {
	if !c.gzipResults {
		return req, nil
	}
	body, err := gzipJSON(req)
	if err != nil {
		return nil, wrapError(err, "gzip result request")
	}
	return body, nil
}

func gzipJSON(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}

	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(raw); err != nil {
		return nil, fmt.Errorf("write gzip body: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close gzip body: %w", err)
	}
	return buf.Bytes(), nil
}

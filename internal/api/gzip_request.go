package api

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func gzipRequestMiddleware(limit int) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		if !strings.EqualFold(strings.TrimSpace(ctx.Get(fiber.HeaderContentEncoding)), "gzip") {
			return ctx.Next()
		}

		body, err := decodeGzipRequestBody(ctx.Request().Body(), limit)
		if err != nil {
			return fiber.NewError(fiber.StatusRequestEntityTooLarge, err.Error())
		}
		ctx.Request().SetBody(body)
		ctx.Request().Header.Del(fiber.HeaderContentEncoding)
		ctx.Request().Header.SetContentLength(len(body))
		return ctx.Next()
	}
}

func decodeGzipRequestBody(raw []byte, limit int) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode gzip request body: %w", err)
	}
	body, readErr := readLimited(reader, limit)
	closeErr := reader.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close gzip request body: %w", closeErr)
	}
	return body, nil
}

func readLimited(reader io.Reader, limit int) ([]byte, error) {
	if limit <= 0 {
		body, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("read gzip request body: %w", err)
		}
		return body, nil
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(io.LimitReader(reader, int64(limit)+1)); err != nil {
		return nil, fmt.Errorf("read gzip request body: %w", err)
	}
	if buf.Len() > limit {
		return nil, fmt.Errorf("request body exceeds %d bytes", limit)
	}
	return buf.Bytes(), nil
}

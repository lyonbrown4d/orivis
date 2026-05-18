package client

import (
	"net"
	"net/http"

	clienthttp "github.com/arcgolabs/clientx/http"
	config "github.com/lyonbrown4d/orivis/internal/agentconfig"
)

func agentHTTPTransportOption(cfg config.Config) clienthttp.Option {
	return func(client *clienthttp.DefaultClient) {
		client.Raw().SetTransport(&http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{}).DialContext,
			MaxIdleConns:          cfg.Transport.MaxIdleConns,
			MaxIdleConnsPerHost:   cfg.Transport.MaxIdleConnsPerHost,
			IdleConnTimeout:       cfg.Transport.IdleConnTimeout,
			TLSHandshakeTimeout:   cfg.Transport.TLSHandshakeTimeout,
			ResponseHeaderTimeout: cfg.Transport.ResponseHeaderTimeout,
		})
	}
}

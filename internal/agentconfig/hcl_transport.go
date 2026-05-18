package config

type agentHCLTransport struct {
	RequestTimeout        string   `hcl:"request_timeout,optional"`
	MaxIdleConns          *int     `hcl:"max_idle_conns,optional"`
	MaxIdleConnsPerHost   *int     `hcl:"max_idle_conns_per_host,optional"`
	IdleConnTimeout       string   `hcl:"idle_conn_timeout,optional"`
	TLSHandshakeTimeout   string   `hcl:"tls_handshake_timeout,optional"`
	ResponseHeaderTimeout string   `hcl:"response_header_timeout,optional"`
	RetryAttempts         *int     `hcl:"retry_attempts,optional"`
	RetryBaseDelay        string   `hcl:"retry_base_delay,optional"`
	RetryMaxDelay         string   `hcl:"retry_max_delay,optional"`
	RetryJitterRatio      *float64 `hcl:"retry_jitter_ratio,optional"`
	GzipResults           *bool    `hcl:"gzip_results,optional"`
}

func (file agentHCLFile) applyTransport(values map[string]any) {
	if file.Transport == nil {
		return
	}
	setString(values, "transport.requesttimeout", file.Transport.RequestTimeout)
	setOptional(values, "transport.maxidleconns", file.Transport.MaxIdleConns)
	setOptional(values, "transport.maxidleconnsperhost", file.Transport.MaxIdleConnsPerHost)
	setString(values, "transport.idleconntimeout", file.Transport.IdleConnTimeout)
	setString(values, "transport.tlshandshaketimeout", file.Transport.TLSHandshakeTimeout)
	setString(values, "transport.responseheadertimeout", file.Transport.ResponseHeaderTimeout)
	setOptional(values, "transport.retryattempts", file.Transport.RetryAttempts)
	setString(values, "transport.retrybasedelay", file.Transport.RetryBaseDelay)
	setString(values, "transport.retrymaxdelay", file.Transport.RetryMaxDelay)
	setOptional(values, "transport.retryjitterratio", file.Transport.RetryJitterRatio)
	setOptional(values, "transport.gzipresults", file.Transport.GzipResults)
}

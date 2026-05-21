package collector

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/lyonbrown4d/orivis/internal/protocol"
)

func ensureAgentResultID(req protocol.AgentResultRequest) protocol.AgentResultRequest {
	if req.ResultID != "" {
		return req
	}
	req.ResultID = newAgentResultID()
	return req
}

func newAgentResultID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "ares_" + hex.EncodeToString(raw[:])
	}
	return "ares_" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

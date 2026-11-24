package errorprivacy

import (
	"bytes"
	"encoding/json"
	"strings"
)

// SSE projects one bounded SSE event, including multiline data. Heartbeats and
// successful events are returned unchanged. The caller owns framing and bounds.
func SSE(event []byte) []byte {
	return SSEWithUpstream(event, true)
}

func SSEWithUpstream(event []byte, upstream bool) []byte {
	return SSEWithProjector(event, upstream, nil)
}

func SSEWithProjector(event []byte, upstream bool, projector *Projector) []byte {
	lines := bytes.Split(event, []byte("\n"))
	var data [][]byte
	for _, line := range lines {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if bytes.HasPrefix(line, []byte("data:")) {
			data = append(data, bytes.TrimPrefix(line[5:], []byte(" ")))
		}
	}
	if len(data) == 0 {
		if SSEType(event) == "error" || SSEType(event) == "response.failed" {
			return SSEFallback()
		}
		return ScrubBytes(event)
	}
	payload := bytes.Join(data, []byte("\n"))
	errorEvent := SSEType(event) == "error" || SSEType(event) == "response.failed"
	updated, errorEnvelope := JSON(payload, upstream)
	changed := errorEnvelope
	if errorEnvelope {
		updated, _ = ScrubJSON(updated)
	} else if projector != nil {
		updated, changed = projector.JSON(payload, upstream)
	} else {
		updated, changed = ScrubJSON(payload)
	}
	if !errorEvent && !errorEnvelope && (json.Valid(payload) || bytes.Equal(bytes.TrimSpace(payload), []byte("[DONE]"))) {
		if !changed {
			return ScrubBytes(event)
		}
		// An identity replacement in a normal delta is not a protocol error.
		// Keep event type, id, comments, retry hints and the original delimiter.
		return replaceSSEData(event, updated)
	}
	if errorEvent && !errorEnvelope {
		updated, changed = Fallback(), true
	}
	if !changed {
		updated = Fallback()
	}
	var out bytes.Buffer
	wroteData := false
	for _, line := range lines {
		if bytes.HasPrefix(line, []byte("data:")) {
			if !wroteData {
				out.WriteString("data: ")
				out.Write(updated)
				out.WriteByte('\n')
				wroteData = true
			}
			continue
		}
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			switch {
			case bytes.HasPrefix(trimmed, []byte("event:")):
				typ := strings.TrimSpace(string(trimmed[6:]))
				if typ != "error" && typ != "response.failed" && typ != "message" {
					line = []byte("event: error")
				}
			case bytes.HasPrefix(trimmed, []byte("id:")):
				id := strings.TrimSpace(string(trimmed[3:]))
				if len(id) > 256 || Text(id) != id {
					continue
				}
			case bytes.HasPrefix(trimmed, []byte("retry:")):
				if strings.Trim(string(trimmed[6:]), " 0123456789") != "" {
					continue
				}
			default:
				// Error-event comments and unknown fields are diagnostic metadata.
				continue
			}
		}
		out.Write(ScrubBytes(line))
		out.WriteByte('\n')
	}
	result := out.Bytes()
	// Split includes a final empty entry; keep exactly the original delimiter.
	if bytes.HasSuffix(event, []byte("\n")) {
		result = result[:len(result)-1]
	}
	return result
}

func replaceSSEData(event, updated []byte) []byte {
	var out bytes.Buffer
	wroteData := false
	for len(event) > 0 {
		line, rest, newline := bytes.Cut(event, []byte("\n"))
		if bytes.HasPrefix(line, []byte("data:")) {
			if !wroteData {
				out.WriteString("data: ")
				out.Write(updated)
				if bytes.HasSuffix(line, []byte("\r")) {
					out.WriteByte('\r')
				}
				if newline {
					out.WriteByte('\n')
				}
				wroteData = true
			}
		} else {
			out.Write(ScrubBytes(line))
			if newline {
				out.WriteByte('\n')
			}
		}
		event = rest
	}
	return out.Bytes()
}

func SSEType(event []byte) string {
	for _, line := range bytes.Split(event, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("event:")) {
			return strings.TrimSpace(string(line[6:]))
		}
	}
	return ""
}

// NormalSSEPrefix permits large normal events to stream without retaining their
// data. Unknown or error events fail closed once they exceed MaxErrorBytes.
func NormalSSEPrefix(prefix []byte) bool {
	if typ := SSEType(prefix); typ != "" {
		return strings.HasPrefix(typ, "response.") && typ != "response.failed" && typ != "response.incomplete" && typ != "response.cancelled" ||
			strings.HasPrefix(typ, "content_block_") || typ == "message_start" || typ == "message_delta" || typ == "message_stop" || typ == "ping"
	}
	for _, line := range bytes.Split(prefix, []byte("\n")) {
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(line[5:])
		// Decode just the first JSON field, so a giant image/delta is never
		// repeatedly parsed or copied while it is still arriving.
		decoder := json.NewDecoder(bytes.NewReader(payload))
		if token, _ := decoder.Token(); token != json.Delim('{') {
			return false
		}
		key, err := decoder.Token()
		if err != nil {
			return false
		}
		if key == "type" || key == "object" {
			var value string
			if decoder.Decode(&value) == nil {
				return value != "error" && value != "response.failed" && value != ""
			}
		}
		// Standard ChatCompletions/Gemini chunks may start with an id/candidates.
		return key == "id" || key == "choices" || key == "candidates"
	}
	return false
}

func SSEFallback() []byte {
	return append(append([]byte("event: error\ndata: "), Fallback()...), '\n', '\n')
}

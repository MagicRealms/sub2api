package errorprivacy

import (
	"strconv"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Projector belongs to one downstream connection. It remembers only short text
// suffixes, so a hidden name split between protocol deltas cannot be recovered
// by concatenating those deltas. It never holds or delays a protocol event.
type Projector struct {
	mu      sync.Mutex
	streams [32]publicTextStream
	next    int
}

type publicTextStream struct {
	key, tail string
}

func (p *Projector) JSON(body []byte, upstream bool) ([]byte, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	projected, changed := PublicJSON(body, upstream)
	typ := gjson.GetBytes(body, "type").String()
	responseID := gjson.GetBytes(body, "response_id").String()
	if len(responseID) > 128 {
		responseID = responseID[:128]
	}
	key := responseID + ":" + strconv.FormatInt(gjson.GetBytes(body, "output_index").Int(), 10) + ":" + strconv.FormatInt(gjson.GetBytes(body, "content_index").Int(), 10)
	apply := func(path, stream string) {
		value := gjson.GetBytes(body, path)
		if value.Type != gjson.String {
			return
		}
		text := value.String()
		streamKey := key + ":" + stream
		var slot *publicTextStream
		for i := range p.streams {
			if p.streams[i].key == streamKey {
				slot = &p.streams[i]
				break
			}
		}
		if slot == nil {
			slot = &p.streams[p.next%len(p.streams)]
			p.next = (p.next + 1) % len(p.streams)
			*slot = publicTextStream{key: streamKey}
		}
		// Inspect only the previous suffix and the next prefix for a boundary
		// match. ScrubJSON already handles complete identities within this delta.
		prefix := text[:min(64, len(text))]
		joined := slot.tail + prefix
		for _, match := range publicShareLLMIdentity.FindAllStringIndex(joined, -1) {
			if match[0] < len(slot.tail) && match[1] > len(slot.tail) {
				end := match[1] - len(slot.tail)
				if updated, err := sjson.SetBytes(projected, path, PublicRouteAlias+ScrubText(text[end:])); err == nil {
					projected, changed = updated, true
				}
				break
			}
		}
		if len(text) >= 32 {
			slot.tail = strings.Clone(text[len(text)-32:])
		} else {
			tail := slot.tail + text
			slot.tail = strings.Clone(tail[max(0, len(tail)-32):])
		}
	}
	switch {
	case typ == "response.output_text.delta" || typ == "response.refusal.delta" || typ == "response.text.delta" || typ == "response.audio_transcript.delta" || typ == "response.output_audio_transcript.delta":
		apply("delta", typ)
	case typ == "content_block_delta":
		apply("delta.text", "anthropic:"+strconv.FormatInt(gjson.GetBytes(body, "index").Int(), 10))
		apply("delta.thinking", "thinking:"+strconv.FormatInt(gjson.GetBytes(body, "index").Int(), 10))
	case typ == "response.completed" || typ == "response.failed" || typ == "message_stop":
		p.streams = [32]publicTextStream{}
	default:
		for i, choice := range gjson.GetBytes(body, "choices").Array() {
			index := strconv.FormatInt(choice.Get("index").Int(), 10)
			for _, field := range []string{"content", "refusal", "reasoning_content"} {
				apply("choices."+strconv.Itoa(i)+".delta."+field, "chat:"+index+":"+field)
			}
		}
		for i, candidate := range gjson.GetBytes(body, "candidates").Array() {
			for j := range candidate.Get("content.parts").Array() {
				apply("candidates."+strconv.Itoa(i)+".content.parts."+strconv.Itoa(j)+".text", "gemini:"+strconv.Itoa(i)+":"+strconv.Itoa(j))
			}
		}
	}
	return projected, changed
}

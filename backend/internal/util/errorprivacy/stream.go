package errorprivacy

import "regexp"

// StreamScrubber covers literal identities split across writes in an oversized
// normal event. Bounded JSON events use ScrubJSON so JSON escapes are decoded.
// It does not buffer a conversation or change event framing.
type StreamScrubber struct {
	tail []byte
}

var streamIdentity = regexp.MustCompile(`(?i)sharellm(?:\.net)?|share[ _-]{1,8}llm`)

func (s *StreamScrubber) Write(body []byte) []byte {
	const keep = 32 // Longer than every literal identity accepted above.
	combined := make([]byte, len(s.tail)+len(body))
	copy(combined, s.tail)
	copy(combined[len(s.tail):], body)
	cut := max(0, len(combined)-keep)
	if hasPrivateIdentityPrefix(combined) {
		for _, match := range streamIdentity.FindAllIndex(combined, -1) {
			if match[0] < cut && match[1] > cut {
				cut = match[0]
				break
			}
		}
	}
	s.tail = append(s.tail[:0], combined[cut:]...)
	return ScrubBytes(combined[:cut])
}

func (s *StreamScrubber) Finish() []byte {
	result := ScrubBytes(s.tail)
	s.tail = nil
	return result
}

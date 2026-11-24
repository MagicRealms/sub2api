package errorprivacy

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestProjectorScrubsIdentitySplitAcrossProtocolDeltas(t *testing.T) {
	for _, protocol := range []string{"responses", "chat", "anthropic", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			var projector Projector
			var text strings.Builder
			for _, delta := range []string{"Visit https://sha", "re", "llm", ".net or https://api.deepseek.com"} {
				quoted, _ := json.Marshal(delta)
				var body, path string
				switch protocol {
				case "responses":
					body, path = `{"type":"response.output_text.delta","response_id":"resp_1","delta":`+string(quoted)+`}`, "delta"
				case "chat":
					body, path = `{"choices":[{"index":0,"delta":{"content":`+string(quoted)+`}}]}`, "choices.0.delta.content"
				case "anthropic":
					body, path = `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":`+string(quoted)+`}}`, "delta.text"
				case "gemini":
					body, path = `{"candidates":[{"content":{"parts":[{"text":`+string(quoted)+`}]}}]}`, "candidates.0.content.parts.0.text"
				}
				projected, _ := projector.JSON([]byte(body), true)
				if !json.Valid(projected) {
					t.Fatal(string(projected))
				}
				text.WriteString(gjson.GetBytes(projected, path).String())
			}
			if strings.Contains(strings.ToLower(text.String()), "sharellm") || !strings.Contains(text.String(), "api.deepseek.com") || !strings.Contains(text.String(), PublicRouteAlias) {
				t.Fatal(text.String())
			}
		})
	}
}

func TestProjectorBoundsAndSeparatesStreams(t *testing.T) {
	var projector Projector
	for i := 0; i < 1000; i++ {
		body := []byte(fmt.Sprintf(`{"type":"response.output_text.delta","response_id":"resp_%d","delta":"share"}`, i))
		projected, _ := projector.JSON(body, true)
		if string(projected) != string(body) {
			t.Fatal("normal fragment changed")
		}
	}
	for _, stream := range projector.streams {
		if len(stream.tail) > 32 || len(stream.key) > 400 {
			t.Fatal("unbounded suffix state")
		}
	}
	other, _ := projector.JSON([]byte(`{"type":"response.output_text.delta","response_id":"other","delta":"llm"}`), true)
	if gjson.GetBytes(other, "delta").String() != "llm" {
		t.Fatal("mixed independent outputs")
	}
}

func TestStreamScrubberPreservesNormalTextAndSplitIdentities(t *testing.T) {
	for _, raw := range []string{"https://sharellm.net/v1", "ShareLLM", "https://example.org/你好"} {
		for chunk := 1; chunk <= len(raw); chunk++ {
			var scrubber StreamScrubber
			var output strings.Builder
			for start := 0; start < len(raw); start += chunk {
				output.Write(scrubber.Write([]byte(raw[start:min(len(raw), start+chunk)])))
				if len(scrubber.tail) > 64 {
					t.Fatal("unbounded stream state")
				}
			}
			output.Write(scrubber.Finish())
			if strings.Contains(strings.ToLower(output.String()), "sharellm") || (strings.Contains(raw, "example.org") && output.String() != raw) {
				t.Fatal(output.String())
			}
		}
	}
}

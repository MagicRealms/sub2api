package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAIStreamEventIsTerminalWithTypeMatchesExistingSemantics(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{name: "empty", data: "", want: false},
		{name: "whitespace", data: " \t ", want: false},
		{name: "done", data: " [DONE] ", want: true},
		{name: "JSON outer whitespace", data: " \n\t {\"type\":\"response.completed\"} \r\n", want: true},
		{name: "completed", data: `{"type":"response.completed"}`, want: true},
		{name: "response done", data: `{"type":"response.done"}`, want: true},
		{name: "failed", data: `{"type":"response.failed"}`, want: true},
		{name: "incomplete", data: `{"type":"response.incomplete"}`, want: true},
		{name: "cancelled", data: `{"type":"response.cancelled"}`, want: true},
		{name: "canceled", data: `{"type":"response.canceled"}`, want: true},
		{name: "delta", data: `{"type":"response.output_text.delta"}`, want: false},
		{name: "invalid JSON", data: `{"type":`, want: false},
		{name: "terminal with trailing garbage", data: `{"type":"response.completed"} trailing`, want: true},
		{name: "nonterminal with trailing garbage", data: `{"type":"response.output_text.delta"} trailing`, want: false},
		{name: "type whitespace is normalized", data: `{"type":" response.completed "}`, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventType := gjson.GetBytes([]byte(tt.data), "type").String()
			got := openAIStreamEventIsTerminalWithType(tt.data, eventType)

			require.Equal(t, tt.want, got)
			require.Equal(t, openAIStreamEventIsTerminal(tt.data), got)
		})
	}
}

func TestNormalizeOpenAIIncompleteDetails(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantChange bool
		wantReason string
	}{
		{
			name:       "event missing reason",
			input:      `{"type":"response.incomplete","response":{"status":"incomplete"}}`,
			wantChange: true,
			wantReason: "max_output_tokens",
		},
		{
			name:       "event null details",
			input:      `{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":null}}`,
			wantChange: true,
			wantReason: "max_output_tokens",
		},
		{
			name:       "response object missing reason",
			input:      `{"status":"incomplete"}`,
			wantChange: true,
			wantReason: "max_output_tokens",
		},
		{
			name:       "preserve provider reason",
			input:      `{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"content_filter"}}}`,
			wantChange: false,
			wantReason: "content_filter",
		},
		{
			name:       "completed unchanged",
			input:      `{"type":"response.completed","response":{"status":"completed"}}`,
			wantChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := normalizeOpenAIIncompleteDetails([]byte(tt.input))
			require.Equal(t, tt.wantChange, changed)
			if tt.wantReason != "" {
				path := "incomplete_details.reason"
				if gjson.GetBytes([]byte(tt.input), "type").String() != "" {
					path = "response." + path
				}
				require.Equal(t, tt.wantReason, gjson.GetBytes(got, path).String())
			}
		})
	}
}

var (
	benchmarkOpenAIResponseSSEEventTypeSink string
	benchmarkOpenAIResponseSSETerminalSink  bool
)

func BenchmarkOpenAIResponseSSETypeExtraction(b *testing.B) {
	data := `{"type":"response.output_text.delta","sequence_number":42,"delta":"streaming response benchmark payload"}`
	dataBytes := []byte(data)

	b.Run("legacy double parse", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(dataBytes)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkOpenAIResponseSSETerminalSink = openAIStreamEventIsTerminal(data)
			benchmarkOpenAIResponseSSEEventTypeSink = strings.TrimSpace(gjson.GetBytes(dataBytes, "type").String())
		}
	})

	b.Run("reused single parse", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(dataBytes)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			eventTypeRaw := gjson.GetBytes(dataBytes, "type").String()
			benchmarkOpenAIResponseSSEEventTypeSink = strings.TrimSpace(eventTypeRaw)
			benchmarkOpenAIResponseSSETerminalSink = openAIStreamEventIsTerminalWithType(data, eventTypeRaw)
		}
	})
}

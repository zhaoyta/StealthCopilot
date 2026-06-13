package intent

import "testing"

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain JSON",
			input: `{"intent":"question"}`,
			want:  `{"intent":"question"}`,
		},
		{
			name:  "markdown code block",
			input: "```json\n{\"intent\":\"followup\"}\n```",
			want:  `{"intent":"followup"}`,
		},
		{
			name:  "markdown no lang",
			input: "```\n{\"intent\":\"statement\"}\n```",
			want:  `{"intent":"statement"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractJSON(tc.input)
			if got != tc.want {
				t.Errorf("extractJSON(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestIntentTypeConstants(t *testing.T) {
	// 确保常量值与 DeepSeek JSON 响应中的字符串一致
	cases := map[IntentType]string{
		IntentQuestion:  "question",
		IntentFollowup:  "followup",
		IntentStatement: "statement",
	}
	for intent, want := range cases {
		if string(intent) != want {
			t.Errorf("IntentType constant %v != %q", intent, want)
		}
	}
}

package init

import "testing"

func TestExtractJSON_WhenVariousShapes_ShouldReturnTheWholeValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "fenced object",
			input: "prose\n```json\n{\"a\":1}\n```\ntrailing",
			want:  `{"a":1}`,
		},
		{
			name:  "bare array",
			input: `[{"index":0},{"index":1}]`,
			want:  `[{"index":0},{"index":1}]`,
		},
		{
			name:  "array after prose",
			input: "Here you go: [{\"index\":0}]",
			want:  `[{"index":0}]`,
		},
		{
			name:  "brace inside string value",
			input: `{"rule":"head :- body }","ok":true}`,
			want:  `{"rule":"head :- body }","ok":true}`,
		},
		{
			name:  "escaped quote inside string value",
			input: `{"quote":"he said \"} \" loudly","ok":true}`,
			want:  `{"quote":"he said \"} \" loudly","ok":true}`,
		},
		{
			name:  "nested object",
			input: `noise {"outer":{"inner":[1,2]}} noise`,
			want:  `{"outer":{"inner":[1,2]}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractJSON(tt.input); got != tt.want {
				t.Errorf("extractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

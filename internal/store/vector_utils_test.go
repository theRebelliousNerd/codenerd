package store

import (
	"reflect"
	"testing"
)

func TestFastParseVectorJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []float32
		wantErr bool
	}{
		{
			name:  "Simple compact",
			input: "[1.1,2.2]",
			want:  []float32{1.1, 2.2},
		},
		{
			name:  "With spaces",
			input: "[ 1.1 , 2.2 ]",
			want:  []float32{1.1, 2.2},
		},
		{
			name:  "Empty array",
			input: "[]",
			want:  []float32{},
		},
		{
			name:  "Single element",
			input: "[1.0]",
			want:  []float32{1.0},
		},
		{
			name:  "Negative and zero",
			input: "[0, -1.5, 0.001]",
			want:  []float32{0, -1.5, 0.001},
		},
		{
			name:    "Invalid format",
			input:   "not json",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fastParseVectorJSON([]byte(tt.input), nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("fastParseVectorJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// Empty slice vs nil check
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("fastParseVectorJSON() = %v, want %v", got, tt.want)
			}
		})
	}
}

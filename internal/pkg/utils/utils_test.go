package utils

import "testing"

func TestCalcSha256Hex(t *testing.T) {

	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"empty string", []byte(""), "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalcSha256Hex(tt.input); got != tt.want {
				t.Errorf("CalcSha256Hex() = %v, want %v", got, tt.want)
			}
		})
	}
}

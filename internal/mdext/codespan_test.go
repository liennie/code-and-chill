package mdext

import (
	"testing"
)

func Test_CodeStringEscape(t *testing.T) {
	tests := []struct {
		s    string
		want string
	}{
		{"foobar", "` foobar `"},
		{"`", "`` ` ``"},
		{"``", "` `` `"},
		{"` `` ``` ````` ``````", "```` ` `` ``` ````` `````` ````"},
	}

	for _, test := range tests {
		t.Run(test.s, func(t *testing.T) {
			if have := CodeStringEscape(test.s); have != test.want {
				t.Fatalf("want: %s, have: %s", test.want, have)
			}
		})
	}
}

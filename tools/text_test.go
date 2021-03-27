package tools

import (
	"regexp"
	"strings"
	"testing"
)

func TestSubdomainRegexp(t *testing.T) {
	t.Parallel()

	examples := []struct {
		text  string
		valid bool
	}{
		{"org", true},
		{"foo-bar-baz-134", true},
		{"ek24Mp13", true},
		{"foo-", false},
		{"-123", false},
		{"foo123", true},
		{"0bar", false},
	}

	for _, example := range examples {
		re := regexp.MustCompile(strings.Join([]string{`\A`, SubdomainRegexp.String(), `\z`}, ""))
		actual := re.MatchString(example.text)
		if actual != example.valid {
			t.Errorf("expected MatchString(`%v`) to be %v; got %v", example.text, example.valid, actual)
		}
	}
}

package repoid

import "testing"

func TestNormalizeForCache(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "trim slashes", input: "/acme/widgets.git/", expected: "acme/widgets.git"},
		{name: "lfs path", input: "/acme/widgets.git/info/lfs", expected: "acme/widgets.git"},
		{name: "empty path", input: "/", expected: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if actual := NormalizeForCache(testCase.input); actual != testCase.expected {
				t.Fatalf("NormalizeForCache(%q) = %q, want %q", testCase.input, actual, testCase.expected)
			}
		})
	}
}

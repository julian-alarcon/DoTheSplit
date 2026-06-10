package service

import "testing"

func TestNeutralizeCSVField(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty stays empty", "", ""},
		{"plain text untouched", "Groceries", "Groceries"},
		{"equals formula defused", "=1+1", "'=1+1"},
		{"plus defused", "+SUM(A1:A2)", "'+SUM(A1:A2)"},
		{"minus defused", "-2+3", "'-2+3"},
		{"at defused", "@SUM(1)", "'@SUM(1)"},
		{"tab prefix defused", "\t=cmd", "'\t=cmd"},
		{"carriage return prefix defused", "\r=cmd", "'\r=cmd"},
		{"hyperlink payload defused", "=HYPERLINK(\"http://x\",\"a\")", "'=HYPERLINK(\"http://x\",\"a\")"},
		{"interior equals untouched", "a=b", "a=b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := neutralizeCSVField(tc.in); got != tc.want {
				t.Fatalf("neutralizeCSVField(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

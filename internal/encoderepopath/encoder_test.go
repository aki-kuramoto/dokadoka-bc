package encoderepopath

import (
	"fmt"
	"testing"
)

func TestEncodeDecode(t *testing.T) {
	cases := []string{
		"https://github.com/user/repo.git",
		"git@github.com:user/repo",
		"ssh://git@example.com:2222/foo/b",
		"日本語/テスト",
		"space and symbols/!*",
		"a-b_c",
		"a_b-c/d.e",
		"simple.repo",
	}

	for _, src := range cases {
		enc := Encode(src)
		fmt.Println(enc)
		dec, err := Decode(enc)
		if err != nil {
			t.Fatalf("decode error for %q: %v", src, err)
		}
		if dec != src {
			t.Fatalf("mismatch: src=%q enc=%q dec=%q", src, enc, dec)
		}
	}
}

func TestInvalidDecode(t *testing.T) {
	cases := []struct {
		form    string
		isValid bool
	}{
		{"-", false},
		{"-1", false},
		{"-ZZ", false},
		{"abc-", false},
		{"https-3A__github.com_user_repo.git", true},
		{"git-40github.com-3Auser_repo", true},
		{"ssh-3A__git-40example.com-3A2222_foo_b", true},
		{"-E6-97-A5-E6-9C-AC-E8-AA-9E_-E3-83-86-E3-82-B9-E3-83-88", true},
		{"space-20and-20symbols_-21-2A", true},
		{"a-2Db-5Fc", true},
		{"a-5Fb-2Dc_d.e", true},
		{"simple.repo", true},
	}

	for _, theCase := range cases {
		_, err := Decode(theCase.form)
		if theCase.isValid {
			if err != nil {
				t.Fatalf("expected no error for %q but %v", theCase.form, err)
			}
		} else {
			if err == nil {
				t.Fatalf("expected error for %q but nil", theCase.form)
			}
		}
	}
}

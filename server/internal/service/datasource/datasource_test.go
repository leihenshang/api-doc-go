package datasource

import "testing"

func TestCheckIdent(t *testing.T) {
	ok := []string{"users", "t_order", "Orders2024", "_tmp"}
	for _, s := range ok {
		if err := checkIdent(s); err != nil {
			t.Errorf("checkIdent(%q) = %v, want nil", s, err)
		}
	}
	bad := []string{"", "a;drop table x", "1abc", "tab le", "tbl`", "a'b", "中文"}
	for _, s := range bad {
		if err := checkIdent(s); err == nil {
			t.Errorf("checkIdent(%q) = nil, want error", s)
		}
	}
}

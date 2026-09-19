package tenant

import "testing"

func TestHasScope(t *testing.T) {
	cases := []struct {
		name     string
		scopes   []string
		required string
		want     bool
	}{
		{"empty set is unrestricted", nil, "chat:completions", true},
		{"wildcard grants all", []string{"*"}, "router:decision", true},
		{"explicit member", []string{"chat:completions", "router:decision"}, "router:decision", true},
		{"explicit non-member", []string{"chat:completions"}, "router:decision", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tn := &Tenant{ID: "tn", Scopes: c.scopes}
			if got := tn.HasScope(c.required); got != c.want {
				t.Errorf("HasScope(%q) with %v = %v, want %v", c.required, c.scopes, got, c.want)
			}
		})
	}
}

func TestHasScopeNilTenant(t *testing.T) {
	var tn *Tenant
	if tn.HasScope("anything") {
		t.Error("nil tenant should grant no scopes")
	}
}

func TestHasRole(t *testing.T) {
	cases := []struct {
		name     string
		role     string
		required string
		want     bool
	}{
		{"legacy role is unrestricted", "", RoleAdmin, true},
		{"admin grants admin", RoleAdmin, RoleAdmin, true},
		{"admin satisfies user", RoleAdmin, RoleUser, true},
		{"user grants user", RoleUser, RoleUser, true},
		{"user does not grant admin", RoleUser, RoleAdmin, false},
		{"unknown role must match exactly", "viewer", RoleAdmin, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tn := &Tenant{ID: "tn", Role: c.role}
			if got := tn.HasRole(c.required); got != c.want {
				t.Errorf("HasRole(%q) with %q = %v, want %v", c.required, c.role, got, c.want)
			}
		})
	}
}

func TestHasRoleNilTenant(t *testing.T) {
	var tn *Tenant
	if tn.HasRole(RoleAdmin) {
		t.Error("nil tenant should grant no roles")
	}
}

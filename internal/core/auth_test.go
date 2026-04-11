package core

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestExpandDisabledToolAliases(t *testing.T) {
	got := expandDisabledToolAliases([]string{"personal_memory", "search_web", "search_memory"})
	want := []string{"search_memory", "read_memory", "read_memory_context", "delete_memory", "search_web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandDisabledToolAliases() = %#v, want %#v", got, want)
	}
}

func TestCollapseDisabledToolsForUI(t *testing.T) {
	got := collapseDisabledToolsForUI([]string{"search_memory", "read_memory_context", "search_web"})
	want := []string{"personal_memory", "search_web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collapseDisabledToolsForUI() = %#v, want %#v", got, want)
	}
}

func TestNewAuthManagerDoesNotCreateDefaultAdmin(t *testing.T) {
	usersFile := filepath.Join(t.TempDir(), "users.json")
	am := NewAuthManager(usersFile)
	if am.HasUsers() {
		t.Fatalf("expected no default users to be created")
	}
}

func TestInitializeAdminCreatesFirstAdminOnlyOnce(t *testing.T) {
	usersFile := filepath.Join(t.TempDir(), "users.json")
	am := NewAuthManager(usersFile)

	if err := am.InitializeAdmin("owner", "secret123"); err != nil {
		t.Fatalf("InitializeAdmin failed: %v", err)
	}
	if !am.HasUsers() {
		t.Fatalf("expected users after initial admin creation")
	}

	user, ok := am.users["owner"]
	if !ok {
		t.Fatalf("expected owner user to exist")
	}
	if user.Role != "admin" {
		t.Fatalf("expected owner role admin, got %q", user.Role)
	}

	if err := am.InitializeAdmin("other", "secret123"); err == nil {
		t.Fatalf("expected second initialization to fail")
	}
}

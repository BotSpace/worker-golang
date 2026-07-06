package config

import "testing"

func TestAppDatabaseURL(t *testing.T) {
	got := appDatabaseURL(Config{
		AppDBHost: "82.25.190.52",
		AppDBPort: "5432",
		AppDBUser: "botspace",
		AppDBPass: "secret/pass",
		AppDBName: "botspace_go",
		AppDBSSL:  "require",
	})
	want := "postgres://botspace:secret%2Fpass@82.25.190.52:5432/botspace_go?sslmode=require"
	if got != want {
		t.Fatalf("appDatabaseURL() = %q, want %q", got, want)
	}
}

func TestAppDatabaseURLMissingRequired(t *testing.T) {
	got := appDatabaseURL(Config{AppDBHost: "postgres", AppDBUser: "botspace"})
	if got != "" {
		t.Fatalf("appDatabaseURL() = %q, want empty", got)
	}
}

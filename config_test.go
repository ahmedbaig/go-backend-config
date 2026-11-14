package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type appConfig struct {
	Port        int      `json:"port" validate:"min=1,max=65535"`
	Host        string   `json:"host" validate:"required"`
	Environment string   `json:"environment" validate:"oneof=localhost testing staging production"`
	Timeout     Duration `json:"timeout"`
}

var defaults = appConfig{Port: 8080, Host: "0.0.0.0", Environment: "production"}

func noEnv(string) (string, bool) { return "", false }

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestEnvKey(t *testing.T) {
	cases := map[string]string{
		"my-app":        "MY_APP_SECRET",
		"my-app-logger": "MY_APP_LOGGER_SECRET",
		"db.main":       "DB_MAIN_SECRET",
	}
	for in, want := range cases {
		if got := EnvKey(in); got != want {
			t.Errorf("EnvKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultsWhenNothingFound(t *testing.T) {
	r, err := LoadWithSource("my-app", defaults, WithDir(t.TempDir()), WithLookup(noEnv))
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != SourceDefault || r.Value != defaults {
		t.Fatalf("got %+v", r)
	}
}

func TestFileOverlaysDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "my-app.json", `{"port": 9000, "timeout": "1500ms"}`)
	r, err := LoadWithSource("my-app", defaults, WithDir(dir), WithLookup(noEnv))
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != SourceFile || r.Value.Port != 9000 || r.Value.Host != "0.0.0.0" {
		t.Fatalf("got %+v", r)
	}
	if r.Value.Timeout.Std() != 1500*time.Millisecond {
		t.Fatalf("timeout = %v", r.Value.Timeout)
	}
}

func TestEnvWinsOverFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "my-app.json", `{"port": 9000}`)
	env := func(k string) (string, bool) {
		if k == "MY_APP_SECRET" {
			return `{"port": 9100}`, true
		}
		return "", false
	}
	r, err := LoadWithSource("my-app", defaults, WithDir(dir), WithLookup(env))
	if err != nil {
		t.Fatal(err)
	}
	if r.Source != SourceEnv || r.Value.Port != 9100 {
		t.Fatalf("got %+v", r)
	}
}

func TestDirFromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(DirEnv, dir)
	writeFile(t, dir, "my-app.json", `{"port": 7000}`)
	v, err := Load("my-app", defaults, WithLookup(noEnv))
	if err != nil {
		t.Fatal(err)
	}
	if v.Port != 7000 {
		t.Fatalf("port = %d", v.Port)
	}
}

func TestValidationTags(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "my-app.json", `{"environment": "prod"}`)
	if _, err := Load("my-app", defaults, WithDir(dir), WithLookup(noEnv)); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestStrictRejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "my-app.json", `{"prot": 1}`)
	if _, err := Load("my-app", defaults, WithDir(dir), WithLookup(noEnv)); err != nil {
		t.Fatalf("lenient mode should accept: %v", err)
	}
	if _, err := Load("my-app", defaults, WithDir(dir), WithLookup(noEnv), WithStrict()); err == nil {
		t.Fatal("strict mode should reject unknown field")
	}
}

func TestBadJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "my-app.json", `{`)
	if _, err := Load("my-app", defaults, WithDir(dir), WithLookup(noEnv)); err == nil {
		t.Fatal("expected decode error")
	}
}

type checked struct {
	Min, Max int
}

var errRange = errors.New("min > max")

func (c checked) Validate() error {
	if c.Min > c.Max {
		return errRange
	}
	return nil
}

func TestValidateMethod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "range.json", `{"Min": 5, "Max": 1}`)
	_, err := Load("range", checked{}, WithDir(dir), WithLookup(noEnv))
	if !errors.Is(err, errRange) {
		t.Fatalf("err = %v", err)
	}
}

func TestErrorDoesNotLeakValues(t *testing.T) {
	env := func(string) (string, bool) { return `{"password": "hunter2", "port": "x"}`, true }
	type withSecret struct {
		Password string `json:"password"`
		Port     int    `json:"port"`
	}
	_, err := Load("svc", withSecret{}, WithDir(t.TempDir()), WithLookup(env))
	if err == nil {
		t.Fatal("expected error")
	}
	if contains(err.Error(), "hunter2") {
		t.Fatalf("error leaks secret: %v", err)
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

func TestDurationJSON(t *testing.T) {
	var d Duration
	if err := d.UnmarshalJSON([]byte(`250`)); err != nil || d.Std() != 250*time.Millisecond {
		t.Fatalf("number: %v %v", d, err)
	}
	if err := d.UnmarshalJSON([]byte(`"2m"`)); err != nil || d.Std() != 2*time.Minute {
		t.Fatalf("string: %v %v", d, err)
	}
	if err := d.UnmarshalJSON([]byte(`"soon"`)); err == nil {
		t.Fatal("expected error")
	}
	b, _ := d.MarshalJSON()
	if string(b) != `"2m0s"` {
		t.Fatalf("marshal = %s", b)
	}
}

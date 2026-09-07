package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rogeecn/email-cli/internal/app"
	"github.com/rogeecn/email-cli/internal/config"
)

type recordingRunner struct {
	options app.Options
}

func (r *recordingRunner) Run(_ context.Context, options app.Options) (app.Result, error) {
	r.options = options
	mode := app.ModeList
	if options.UID != 0 {
		mode = app.ModeDetail
	}
	return app.Result{Mode: mode, Format: "json"}, nil
}

func setupMailClaw(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	path := config.DefaultPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	content := fmt.Sprintf("default_account = \"cloud\"\n[accounts.cloud]\nprovider = \"mailclaw\"\n[accounts.cloud.mailclaw]\nhost = %q\napi_token = \"test-secret\"\n", server.URL)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return server.URL
}

func invokeMailClaw(args ...string) (int, string, string) {
	var out, err bytes.Buffer
	code := Main(args, &out, &err)
	return code, out.String(), err.String()
}

func TestMailClawTOMLConfigAndRoutes(t *testing.T) {
	var method, path, query string
	setupMailClaw(t, func(w http.ResponseWriter, r *http.Request) {
		method, path, query = r.Method, r.URL.Path, r.URL.RawQuery
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("missing saved token")
		}
		fmt.Fprint(w, `{"success":true,"data":{"id":"uuid-123","total":1}}`)
	})
	for _, tc := range []struct {
		args         []string
		method, path string
	}{
		{[]string{"list", "--q", "hello & world", "--limit", "2", "--offset", "3", "--after", "2026-01-01", "--before", "2026-02-01", "--from", "sender", "--to", "recipient"}, "GET", "/api/emails"},
		{[]string{"export"}, "GET", "/api/emails/export"},
		{[]string{"get", "uuid-123"}, "GET", "/api/emails/uuid-123"},
		{[]string{"attachments", "uuid-123"}, "GET", "/api/emails/uuid-123/attachments"},
		{[]string{"delete", "uuid-123", "--yes"}, "DELETE", "/api/emails/uuid-123"},
		{[]string{"health"}, "GET", "/api/health"},
	} {
		code, out, stderr := invokeMailClaw(append(tc.args, "--format", "json")...)
		if code != 0 || stderr != "" || !json.Valid([]byte(out)) {
			t.Fatalf("%v: %d %s %s", tc.args, code, out, stderr)
		}
		if method != tc.method || path != tc.path {
			t.Fatalf("%v: %s %s", tc.args, method, path)
		}
		if tc.args[0] == "list" && (!strings.Contains(query, "limit=2") || !strings.Contains(query, "offset=3") || !strings.Contains(query, "q=hello+%26+world") || !strings.Contains(query, "after=1767225600")) {
			t.Errorf("incorrect filters: %s", query)
		}
	}
}

func TestMailClawSendContract(t *testing.T) {
	var body map[string]any
	setupMailClaw(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/emails/send" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("wrong send route: %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		fmt.Fprint(w, `{"success":true,"data":{"id":"sent-id"}}`)
	})
	file := filepath.Join(t.TempDir(), "body.txt")
	if err := os.WriteFile(file, []byte("你好\nbody"), 0600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := invokeMailClaw("send", "--from", "Sender <s@example.com>", "--to", "a@example.com", "--to", "b@example.com", "--cc", "c@example.com", "--bcc", "d@example.com", "--reply-to", "r@example.com", "--subject", "test", "--text-file", file, "--html", "<p>body</p>", "--header", "X-Test=value=extra", "--tag", "type=test", "--scheduled-at", "2027-01-01T00:00:00Z")
	if code != 0 {
		t.Fatal(stderr)
	}
	if len(body["to"].([]any)) != 2 || body["text"] != "你好\nbody" || body["html"] != "<p>body</p>" || body["reply_to"].([]any)[0] != "r@example.com" || body["headers"].(map[string]any)["X-Test"] != "value=extra" || body["tags"].([]any)[0].(map[string]any)["name"] != "type" {
		t.Fatalf("bad payload: %#v", body)
	}
}

func TestMailClawInvalidCommandsNeverReachServer(t *testing.T) {
	setupMailClaw(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("invalid request reached server: %s %s", r.Method, r.URL.Path)
	})
	for _, args := range [][]string{
		{"delete", "id"}, {"delete", "id", "--yes=false"}, {"get", "../send"}, {"get"}, {"get", "id", "extra"},
		{"list", "--limit", "0"}, {"list", "--limit", "101"}, {"list", "--offset", "-1"}, {"list", "--after", "bad"}, {"list", "--after", "200", "--before", "100"},
		{"list", "--mailbox", "INBOX"}, {"list", "--format", "xml"}, {"list", "--q"}, {"get", "id", "--limit", "2"},
		{"download", "id", "aid"}, {"send", "--to", "a@example.com", "--subject", "test", "--text", "hi"},
		{"send", "--from", "a@example.com", "--to", "invalid", "--subject", "test", "--text", "hi"},
		{"send", "--from", "a@example.com", "--to", "b@example.com", "--subject", "test"},
		{"send", "--from", "a@example.com", "--to", "b@example.com", "--subject", "bad\nheader", "--text", "hi"},
		{"list", "--config", "/tmp/unused.toml"}, {"unknown"},
		{"list", "--mailclaw-config", "legacy.json"}, {"config", "path"}, {"config", "show"}, {"config", "set"},
	} {
		code, _, stderr := invokeMailClaw(args...)
		if code == 0 || stderr == "" {
			t.Errorf("accepted invalid args %v", args)
		}
		if strings.Contains(stderr, "test-secret") {
			t.Errorf("leaked secret: %s", stderr)
		}
	}
}

func TestMailClawExportAndDownload(t *testing.T) {
	setupMailClaw(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/attachments/") {
			fmt.Fprint(w, "binary\x00")
			return
		}
		fmt.Fprint(w, `{"success":true,"data":{"emails":[{"id":"id","text_body":"full body"}],"total":10,"limit":1,"offset":0}}`)
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "page.json")
	code, out, stderr := invokeMailClaw("export", "--format", "json", "--limit", "1", "--output", path)
	if code != 0 || out != "" {
		t.Fatalf("export: %d %q %s", code, out, stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil || !json.Valid(data) || !strings.Contains(string(data), "full body") {
		t.Fatalf("export bytes: %s %v", data, err)
	}
	if code, _, _ := invokeMailClaw("export", "-o", path); code == 0 {
		t.Fatal("export overwrote file")
	}
	attachment := filepath.Join(dir, "attachment.bin")
	if code, _, stderr := invokeMailClaw("download", "id", "aid", "-o", attachment); code != 0 {
		t.Fatal(stderr)
	}
	data, _ = os.ReadFile(attachment)
	if string(data) != "binary\x00" {
		t.Fatalf("download: %q", data)
	}
}

func TestMailClawNamedAccountsAndDefaultRootRouting(t *testing.T) {
	var lastPath string
	host := setupMailClaw(t, func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		fmt.Fprint(w, `{"success":true,"data":{"emails":[],"id":"string-id","total":0}}`)
	})
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := fmt.Sprintf("default_account = \"cloud\"\n[accounts.cloud]\nprovider = \"mailclaw\"\n[accounts.cloud.mailclaw]\nhost = %q\napi_token = \"test-secret\"\n[accounts.cloud.defaults]\nformat = \"json\"\npage_size = 5\n[accounts.imap]\nprovider = \"gmail\"\n", host)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"list", "-c", path, "-A", "cloud"}, {"list", "-c", path}} {
		if code, out, stderr := invokeMailClaw(args...); code != 0 || !json.Valid([]byte(out)) {
			t.Fatalf("TOML account %v: %s %s", args, out, stderr)
		}
	}
	for _, args := range [][]string{{"-c", path}, {"-c", path, "-A", "cloud", "--id", "string-id"}} {
		var out, stderr bytes.Buffer
		if code := Main(args, &out, &stderr); code != 0 || !json.Valid(out.Bytes()) {
			t.Fatalf("root %v: %d %s %s", args, code, &out, &stderr)
		}
	}
	if lastPath != "/api/emails/string-id" {
		t.Errorf("string ID routing: %s", lastPath)
	}
	for _, args := range [][]string{{"-c", path, "--uid", "1"}, {"-c", path, "--mailbox", "INBOX"}} {
		var out, stderr bytes.Buffer
		if code := Main(args, &out, &stderr); code == 0 {
			t.Errorf("accepted IMAP options %v", args)
		}
	}
	if code, _, _ := invokeMailClaw("delete", "id", "--yes", "-c", path, "-A", "imap"); code == 0 {
		t.Fatal("accepted mutation for IMAP account")
	}
	if code, _, _ := invokeMailClaw("list", "--limit", "0", "-c", path, "-A", "cloud"); code == 0 {
		t.Fatal("account default hid invalid explicit limit")
	}
}

func TestMailClawHelpAndRunDispatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, args := range [][]string{{"--help"}, {"send", "--help"}} {
		code, out, stderr := invokeMailClaw(args...)
		if code != 0 || out+stderr == "" || strings.Contains(out+stderr, "test-secret") {
			t.Fatalf("%v: %d %s %s", args, code, out, stderr)
		}
	}
	var out, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"send", "--help"}, &out, &stderr, nil); code != 0 || !strings.Contains(stderr.String(), "download") {
		t.Fatalf("Run dispatch %d %s", code, &stderr)
	}
	out.Reset()
	stderr.Reset()
	if code := Main([]string{"mailclaw", "list"}, &out, &stderr); code == 0 || !strings.Contains(stderr.String(), "prefix was removed") {
		t.Fatalf("legacy prefix accepted: %d %s", code, &stderr)
	}
}

func TestMailClawUTF8Bodies(t *testing.T) {
	setupMailClaw(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid body must not access API") })
	dir := t.TempDir()
	file := filepath.Join(dir, "body.txt")
	if err := os.WriteFile(file, []byte{0xff}, 0600); err != nil {
		t.Fatal(err)
	}
	base := []string{"send", "--from", "a@example.com", "--to", "b@example.com", "--subject", "test"}
	for _, body := range [][]string{
		{"--text-file", file},
		{"--text", "inline", "--text-file", file},
		{"--text-file", filepath.Join(dir, "missing.txt")},
		{"--text", "body", "--header", "X-Test=bad\nheader"},
		{"--text", "body", "--tag", "empty="},
		{"--text", "body", "--scheduled-at", "tomorrow"},
	} {
		args := append(append([]string{}, base...), body...)
		if code, _, stderr := invokeMailClaw(args...); code == 0 {
			t.Fatalf("accepted %v: %s", body, stderr)
		}
	}
}

func TestMailClawRequiresInlineCredentials(t *testing.T) {
	host := setupMailClaw(t, func(w http.ResponseWriter, r *http.Request) { t.Error("must not fall back to legacy credentials") })
	legacy := filepath.Join(os.Getenv("HOME"), ".mailclaw", "config.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]string{"host": host, "api_token": "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MAILCLAW_CONFIG", legacy)
	t.Setenv("MAILCLAW_HOST", host)
	t.Setenv("MAILCLAW_API_TOKEN", "test-secret")
	for _, fields := range []string{
		fmt.Sprintf("host = %q", host),
		"api_token = \"test-secret\"",
		fmt.Sprintf("config = %q", legacy),
	} {
		content := "default_account = \"cloud\"\n[accounts.cloud]\nprovider = \"mailclaw\"\n[accounts.cloud.mailclaw]\n" + fields
		if err := os.WriteFile(config.DefaultPath(), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		for _, args := range [][]string{{"mailclaw", "list"}, {"--id", "string-id"}} {
			var out, stderr bytes.Buffer
			if code := Main(args, &out, &stderr); code == 0 || strings.Contains(out.String()+stderr.String(), "test-secret") {
				t.Fatalf("missing inline credentials: code=%d out=%s err=%s", code, &out, &stderr)
			}
		}
	}
}

func TestUnifiedCommandsDispatchIMAPByAccount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "default_account = \"imap\"\n[accounts.imap]\nprovider = \"selfhost\"\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	runner := &recordingRunner{}
	factory := func() Runner { return runner }
	for _, tc := range []struct {
		args []string
		uid  uint32
	}{
		{[]string{"list", "-c", path, "-A", "imap", "--limit", "5", "--mailbox", "Archive"}, 0},
		{[]string{"get", "4294967295", "-c", path, "-A", "imap"}, 4294967295},
	} {
		var out, stderr bytes.Buffer
		if code := Run(context.Background(), tc.args, &out, &stderr, factory); code != 0 {
			t.Fatalf("%v: %d %s", tc.args, code, &stderr)
		}
		if runner.options.UID != tc.uid || runner.options.Account != "imap" {
			t.Fatalf("wrong dispatch for %v: %#v", tc.args, runner.options)
		}
	}
	for _, args := range [][]string{{"get", "not-a-uid", "-c", path}, {"send", "-c", path}} {
		var out, stderr bytes.Buffer
		if code := Run(context.Background(), args, &out, &stderr, factory); code == 0 {
			t.Fatalf("unsupported IMAP command accepted: %v", args)
		}
	}
}

func TestRootHelpSucceedsWithoutConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, help := range []string{"--help", "-h"} {
		var out, stderr bytes.Buffer
		if code := Main([]string{help}, &out, &stderr); code != 0 || !strings.Contains(out.String()+stderr.String(), "MailClaw") {
			t.Fatalf("help: code=%d out=%s err=%s", code, &out, &stderr)
		}
		out.Reset()
		stderr.Reset()
		if code := Run(context.Background(), []string{help}, &out, &stderr, nil); code != 0 {
			t.Fatalf("Run help: %d %s", code, &stderr)
		}
	}
}

func TestRootFlagValidation(t *testing.T) {
	for _, args := range [][]string{{"--uid", "4294967296"}, {"--uid", "0"}, {"--limit", "0"}, {"--offset", "-1"}, {"--id", ""}, {"--id", "uuid", "--uid", "1"}, {"typo"}} {
		if _, err := ParseFlags(args); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	if options, err := ParseFlags([]string{"--uid", "4294967295"}); err != nil || options.UID != 4294967295 {
		t.Fatalf("valid UID rejected: %v", err)
	}
}

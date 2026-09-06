package mailclaw

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/base/api/emails" || r.URL.Query().Get("q") != "one & two" || r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Errorf("unexpected request: path=%s query=%v", r.URL.Path, r.URL.Query())
		}
		fmt.Fprint(w, `{"success":true,"data":{"emails":[{"id":"uuid-not-a-number"}],"total":1}}`)
	}))
	defer server.Close()
	client, err := New(server.URL+"/base/", "test-secret")
	if err != nil {
		t.Fatal(err)
	}
	data, err := client.Do(context.Background(), http.MethodGet, "/api/emails", url.Values{"q": {"one & two"}}, nil)
	if err != nil || !strings.Contains(string(data), `"uuid-not-a-number"`) {
		t.Fatalf("data=%s error=%v", data, err)
	}
}

func TestClientRejectsUnsafeInput(t *testing.T) {
	for _, host := range []string{"", "http://example.com", "ftp://localhost", "https://u:p@example.com", "https://example.com?q=1", "https://example.com?", "https://example.com/#part", ":bad"} {
		if _, err := New(host, "secret"); err == nil {
			t.Errorf("accepted host %q", host)
		}
	}
	for _, token := range []string{"", "a\nb", "a b", "a\x00b"} {
		if _, err := New("https://example.com", token); err == nil {
			t.Errorf("accepted unsafe token")
		}
	}
	for _, id := range []string{"", ".", "..", "../send", "a/b", "a\\b", "a?b", "a#b", "%2e%2e", "a\nb", " space"} {
		if _, err := EmailPath(id); err == nil {
			t.Errorf("accepted ID %q", id)
		}
	}
}

func TestClientErrorsNeverReflectSecrets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{"unauthorized", 401, `secret-token`},
		{"server failure", 500, `secret-token`},
		{"invalid JSON", 200, `secret-token`},
		{"API failure", 200, `{"success":false,"error":{"message":"secret-token"}}`},
		{"missing data", 200, `{"success":true}`},
		{"null data", 200, `{"success":true,"data":null}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			client, err := New(server.URL, "secret-token")
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Do(context.Background(), "GET", "/api/emails", nil, nil)
			if err == nil || strings.Contains(err.Error(), "secret-token") {
				t.Fatalf("unsafe error: %v", err)
			}
		})
	}
}

func TestClientRejectsRedirectAndCancellation(t *testing.T) {
	var redirected bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer server.Close()
	client, err := New(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Do(context.Background(), "POST", "/api/emails/send", nil, map[string]string{"text": "test"}); err == nil {
		t.Fatal("redirect accepted")
	}
	if redirected {
		t.Fatal("redirect leaked credentials or replayed send")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Do(ctx, "GET", "/api/emails", nil, nil); err != context.Canceled {
		t.Fatalf("cancel error: %v", err)
	}
}

func TestDownloadAtomicAndNoOverwrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/emails/email-id/attachments/attachment-id" {
			t.Errorf("wrong path %s", r.URL.Path)
		}
		w.Header().Set("Content-Disposition", `attachment; filename="../../escape"`)
		w.Write([]byte{0, 1, 2, 255})
	}))
	defer server.Close()
	client, err := New(server.URL, "secret")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "saved.bin")
	n, err := client.Download(context.Background(), "email-id", "attachment-id", path)
	if err != nil || n != 4 {
		t.Fatalf("download: n=%d err=%v", n, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != string([]byte{0, 1, 2, 255}) {
		t.Fatalf("bad bytes %v %v", data, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Errorf("unsafe permissions: %v", info.Mode())
	}
	if _, err := client.Download(context.Background(), "email-id", "attachment-id", path); err == nil {
		t.Fatal("overwrote existing file")
	}
	link := filepath.Join(dir, "symlink")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Download(context.Background(), "email-id", "attachment-id", link); err == nil {
		t.Fatal("followed output symlink")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("unexpected temporary/remote files: %v", entries)
	}
}

func TestDownloadFailureLeavesNoFile(t *testing.T) {
	for _, failure := range []string{"truncated", "http"} {
		t.Run(failure, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if failure == "http" {
					w.WriteHeader(404)
					return
				}
				w.Header().Set("Content-Length", "100")
				fmt.Fprint(w, "short")
			}))
			defer server.Close()
			client, err := New(server.URL, "secret")
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			if _, err := client.Download(context.Background(), "email", "attachment", filepath.Join(dir, "out")); err == nil {
				t.Fatal("failed download succeeded")
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Fatalf("partial files left: %v", entries)
			}
		})
	}
}

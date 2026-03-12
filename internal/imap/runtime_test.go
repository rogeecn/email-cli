package imap

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	imapv2 "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/rogeecn/email-cli/internal/config"
)

type fakeSessionClient struct {
	loginUsername   string
	loginPassword   string
	selectedMailbox string
	searched        bool
	searchResult    []imapv2.UID
	searchErr       error
	fetchMessages   []*fakeFetchMessage
	fetchErr        error
}

type fakeLoginCommand struct{ err error }

func (c fakeLoginCommand) Wait() error { return c.err }

type fakeSelectCommand struct {
	data *imapv2.SelectData
	err  error
}

func (c fakeSelectCommand) Wait() (*imapv2.SelectData, error) { return c.data, c.err }

type fakeSearchCommand struct {
	data *imapv2.SearchData
	err  error
}

func (c fakeSearchCommand) Wait() (*imapv2.SearchData, error) { return c.data, c.err }

type fakeFetchCommand struct {
	messages []*fakeFetchMessage
	err      error
	index    int
}

func (c *fakeFetchCommand) Next() fetchMessage {
	if c.index >= len(c.messages) {
		return nil
	}
	msg := c.messages[c.index]
	c.index++
	return msg
}
func (c *fakeFetchCommand) Close() error { return c.err }

type fakeFetchMessage struct {
	items []fetchItem
	index int
}

func (m *fakeFetchMessage) Next() fetchItem {
	if m.index >= len(m.items) {
		return nil
	}
	item := m.items[m.index]
	m.index++
	return item
}

type fakeFetchBodySection struct{ literal io.Reader }

func (f fakeFetchBodySection) body() io.Reader { return f.literal }

type fakeFetchUID struct{ uid uint32 }

func (f fakeFetchUID) value() uint32 { return f.uid }

type fakeFetchFlags struct{ flags []string }

func (f fakeFetchFlags) values() []string { return f.flags }

func (f *fakeSessionClient) Login(username, password string) loginCommand {
	f.loginUsername = username
	f.loginPassword = password
	return fakeLoginCommand{}
}

func (f *fakeSessionClient) Select(mailbox string) selectCommand {
	f.selectedMailbox = mailbox
	return fakeSelectCommand{data: &imapv2.SelectData{NumMessages: 2}}
}

func (f *fakeSessionClient) UIDSearch() searchCommand {
	f.searched = true
	return fakeSearchCommand{data: &imapv2.SearchData{All: imapv2.UIDSetNum(f.searchResult...)}, err: f.searchErr}
}

func (f *fakeSessionClient) Fetch(uids []uint32, _ bool) fetchCommand {
	allowed := make(map[uint32]struct{}, len(uids))
	for _, uid := range uids {
		allowed[uid] = struct{}{}
	}

	messages := make([]*fakeFetchMessage, 0, len(f.fetchMessages))
	for _, message := range f.fetchMessages {
		message.index = 0
		for _, item := range message.items {
			typed, ok := item.(fakeFetchUID)
			if !ok {
				continue
			}
			if _, exists := allowed[typed.uid]; exists {
				messages = append(messages, message)
			}
			break
		}
	}
	return &fakeFetchCommand{messages: messages, err: f.fetchErr}
}

func (f *fakeSessionClient) Close() error  { return nil }
func (f *fakeSessionClient) Logout() error { return nil }

func TestRuntimeClientListsRecentMessages(t *testing.T) {
	raw1 := "From: Alice <alice@example.com>\r\nTo: Bob <bob@example.com>\r\nSubject: First\r\nDate: Thu, 12 Mar 2026 10:00:00 +0000\r\n\r\nOne"
	raw2 := "From: Carol <carol@example.com>\r\nTo: Dan <dan@example.com>\r\nSubject: Second\r\nDate: Thu, 12 Mar 2026 11:00:00 +0000\r\n\r\nTwo"
	raw3 := "From: Eve <eve@example.com>\r\nTo: Frank <frank@example.com>\r\nSubject: Third\r\nDate: Thu, 12 Mar 2026 12:00:00 +0000\r\n\r\nThree"
	session := &fakeSessionClient{
		searchResult: []imapv2.UID{10, 20, 30},
		fetchMessages: []*fakeFetchMessage{
			{items: []fetchItem{fakeFetchUID{uid: 10}, fakeFetchFlags{flags: []string{"\\Seen"}}, fakeFetchBodySection{literal: strings.NewReader(raw1)}}},
			{items: []fetchItem{fakeFetchUID{uid: 20}, fakeFetchFlags{flags: []string{}}, fakeFetchBodySection{literal: strings.NewReader(raw2)}}},
			{items: []fetchItem{fakeFetchUID{uid: 30}, fakeFetchFlags{flags: []string{}}, fakeFetchBodySection{literal: strings.NewReader(raw3)}}},
		},
	}
	client := NewRuntimeClient(func(account config.AccountConfig) (sessionClient, error) {
		return session, nil
	})

	listResult, err := client.ListRecent(context.Background(), config.AccountConfig{
		Auth: config.AuthConfig{Username: "user@example.com", Password: "secret"},
	}, "INBOX", 2, 1)
	if err != nil {
		t.Fatalf("ListRecent returned error: %v", err)
	}
	if session.loginUsername != "user@example.com" || session.loginPassword != "secret" {
		t.Fatalf("login used wrong credentials: %q %q", session.loginUsername, session.loginPassword)
	}
	if session.selectedMailbox != "INBOX" {
		t.Fatalf("selected mailbox = %q, want INBOX", session.selectedMailbox)
	}
	if !session.searched {
		t.Fatalf("expected UIDSearch to be called")
	}
	if len(listResult.Summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(listResult.Summaries))
	}
	if listResult.Total != 3 {
		t.Fatalf("total = %d, want 3", listResult.Total)
	}
	if listResult.Summaries[0].UID != 20 || listResult.Summaries[0].Subject != "Second" {
		t.Fatalf("latest summary = %+v", listResult.Summaries[0])
	}
	if listResult.Summaries[1].UID != 10 || !listResult.Summaries[1].Seen {
		t.Fatalf("older summary = %+v", listResult.Summaries[1])
	}
}

func TestRuntimeClientGetsMessageByUID(t *testing.T) {
	raw := strings.Join([]string{
		"From: Alice <alice@example.com>",
		"To: Bob <bob@example.com>",
		"Subject: Hello",
		"Date: Thu, 12 Mar 2026 10:00:00 +0000",
		"",
		"Body",
	}, "\r\n")
	session := &fakeSessionClient{
		fetchMessages: []*fakeFetchMessage{
			{items: []fetchItem{fakeFetchUID{uid: 42}, fakeFetchFlags{flags: []string{"\\Seen"}}, fakeFetchBodySection{literal: strings.NewReader(raw)}}},
		},
	}
	client := NewRuntimeClient(func(account config.AccountConfig) (sessionClient, error) {
		return session, nil
	})

	detail, err := client.GetByUID(context.Background(), config.AccountConfig{
		Auth: config.AuthConfig{Username: "user@example.com", Password: "secret"},
	}, "INBOX", 42)
	if err != nil {
		t.Fatalf("GetByUID returned error: %v", err)
	}
	if detail.UID != 42 || detail.Subject != "Hello" {
		t.Fatalf("unexpected detail: %+v", detail)
	}
}

func TestRuntimeClientListRecentSkipsUnknownCharsetMessages(t *testing.T) {
	gbkRaw := strings.Join([]string{
		"From: Sender <sender@example.com>",
		"To: Bob <bob@example.com>",
		"Subject: =?GBK?B?1eLKx9bQ?=",
		"Date: Thu, 12 Mar 2026 10:00:00 +0000",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=gbk",
		"",
		"test",
	}, "\r\n")
	goodRaw := strings.Join([]string{
		"From: Carol <carol@example.com>",
		"To: Dan <dan@example.com>",
		"Subject: Second",
		"Date: Thu, 12 Mar 2026 11:00:00 +0000",
		"",
		"Two",
	}, "\r\n")
	var logs bytes.Buffer
	session := &fakeSessionClient{
		searchResult: []imapv2.UID{10, 20},
		fetchMessages: []*fakeFetchMessage{
			{items: []fetchItem{fakeFetchUID{uid: 10}, fakeFetchFlags{flags: []string{}}, fakeFetchBodySection{literal: strings.NewReader(gbkRaw)}}},
			{items: []fetchItem{fakeFetchUID{uid: 20}, fakeFetchFlags{flags: []string{"\\Seen"}}, fakeFetchBodySection{literal: strings.NewReader(goodRaw)}}},
		},
	}
	client := NewRuntimeClient(func(account config.AccountConfig) (sessionClient, error) {
		return session, nil
	}).WithDebugOutput(&logs)

	listResult, err := client.ListRecent(context.Background(), config.AccountConfig{
		Provider: "qq",
		Auth:     config.AuthConfig{Username: "user@example.com", Password: "secret"},
		IMAP:     config.IMAPConfig{Host: "imap.qq.com", Port: 993, TLS: true},
	}, "INBOX", 20, 0)
	if err != nil {
		t.Fatalf("ListRecent returned error: %v", err)
	}
	if len(listResult.Summaries) != 1 {
		t.Fatalf("len(summaries) = %d, want 1", len(listResult.Summaries))
	}
	if listResult.Total != 2 {
		t.Fatalf("total = %d, want 2", listResult.Total)
	}
	if listResult.Summaries[0].Subject != "Second" {
		t.Fatalf("Subject = %q, want %q", listResult.Summaries[0].Subject, "Second")
	}
	if !strings.Contains(logs.String(), "parse message skipped") {
		t.Fatalf("debug logs should mention skipped parse error, got %q", logs.String())
	}
}

func TestRuntimeClientPropagatesDialErrors(t *testing.T) {
	expectedErr := errors.New("dial failed")
	client := NewRuntimeClient(func(account config.AccountConfig) (sessionClient, error) {
		return nil, expectedErr
	})

	_, err := client.ListRecent(context.Background(), config.AccountConfig{}, "INBOX", 1, 0)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("ListRecent error = %v, want %v", err, expectedErr)
	}
}

func TestRuntimeClientWritesDebugLogs(t *testing.T) {
	raw := "From: Alice <alice@example.com>\r\nTo: Bob <bob@example.com>\r\nSubject: First\r\nDate: Thu, 12 Mar 2026 10:00:00 +0000\r\n\r\nOne"
	session := &fakeSessionClient{
		searchResult: []imapv2.UID{10},
		fetchMessages: []*fakeFetchMessage{
			{items: []fetchItem{fakeFetchUID{uid: 10}, fakeFetchFlags{flags: []string{"\\Seen"}}, fakeFetchBodySection{literal: strings.NewReader(raw)}}},
		},
	}
	var logs bytes.Buffer
	client := NewRuntimeClient(func(account config.AccountConfig) (sessionClient, error) {
		return session, nil
	})
	client = client.WithDebugOutput(&logs)

	_, err := client.ListRecent(context.Background(), config.AccountConfig{
		Provider: "qq",
		Auth:     config.AuthConfig{Username: "user@example.com", Password: "secret"},
		IMAP:     config.IMAPConfig{Host: "imap.qq.com", Port: 993, TLS: true},
	}, "INBOX", 1, 0)
	if err != nil {
		t.Fatalf("ListRecent returned error: %v", err)
	}

	text := logs.String()
	if !strings.Contains(text, "imap connect") {
		t.Fatalf("debug logs should include connect stage, got %q", text)
	}
	if !strings.Contains(text, "imap login ok") {
		t.Fatalf("debug logs should include login stage, got %q", text)
	}
	if !strings.Contains(text, "imap search returned 1 uid") {
		t.Fatalf("debug logs should include search result, got %q", text)
	}
}

func TestUIDSearchCriteriaFetchesAllMessages(t *testing.T) {
	criteria := uidSearchCriteria()
	if criteria == nil {
		t.Fatalf("criteria should not be nil")
	}
	if len(criteria.UID) != 0 {
		t.Fatalf("UID criteria should be empty to fetch all messages, got %+v", criteria.UID)
	}
}

func TestWrapIMAPClientUsesRealClientInterfaces(t *testing.T) {
	_ = wrapClient((*imapclient.Client)(nil))
}

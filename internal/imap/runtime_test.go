package imap

import (
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

func (f *fakeSessionClient) Fetch(_ []uint32, _ bool) fetchCommand {
	return &fakeFetchCommand{messages: f.fetchMessages, err: f.fetchErr}
}

func (f *fakeSessionClient) Close() error  { return nil }
func (f *fakeSessionClient) Logout() error { return nil }

func TestRuntimeClientListsRecentMessages(t *testing.T) {
	raw1 := "From: Alice <alice@example.com>\r\nTo: Bob <bob@example.com>\r\nSubject: First\r\nDate: Thu, 12 Mar 2026 10:00:00 +0000\r\n\r\nOne"
	raw2 := "From: Carol <carol@example.com>\r\nTo: Dan <dan@example.com>\r\nSubject: Second\r\nDate: Thu, 12 Mar 2026 11:00:00 +0000\r\n\r\nTwo"
	session := &fakeSessionClient{
		searchResult: []imapv2.UID{10, 20},
		fetchMessages: []*fakeFetchMessage{
			{items: []fetchItem{fakeFetchUID{uid: 10}, fakeFetchFlags{flags: []string{"\\Seen"}}, fakeFetchBodySection{literal: strings.NewReader(raw1)}}},
			{items: []fetchItem{fakeFetchUID{uid: 20}, fakeFetchFlags{flags: []string{}}, fakeFetchBodySection{literal: strings.NewReader(raw2)}}},
		},
	}
	client := NewRuntimeClient(func(account config.AccountConfig) (sessionClient, error) {
		return session, nil
	})

	summaries, err := client.ListRecent(context.Background(), config.AccountConfig{
		Auth: config.AuthConfig{Username: "user@example.com", Password: "secret"},
	}, "INBOX", 2)
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
	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}
	if summaries[0].UID != 20 || summaries[0].Subject != "Second" {
		t.Fatalf("latest summary = %+v", summaries[0])
	}
	if summaries[1].UID != 10 || !summaries[1].Seen {
		t.Fatalf("older summary = %+v", summaries[1])
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

func TestRuntimeClientPropagatesDialErrors(t *testing.T) {
	expectedErr := errors.New("dial failed")
	client := NewRuntimeClient(func(account config.AccountConfig) (sessionClient, error) {
		return nil, expectedErr
	})

	_, err := client.ListRecent(context.Background(), config.AccountConfig{}, "INBOX", 1)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("ListRecent error = %v, want %v", err, expectedErr)
	}
}

func TestWrapIMAPClientUsesRealClientInterfaces(t *testing.T) {
	_ = wrapClient((*imapclient.Client)(nil))
}

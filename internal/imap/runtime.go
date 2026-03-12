package imap

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"

	imapv2 "github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/rogeecn/email-cli/internal/config"
	"github.com/rogeecn/email-cli/internal/mail"
)

type loginCommand interface {
	Wait() error
}

type selectCommand interface {
	Wait() (*imapv2.SelectData, error)
}

type searchCommand interface {
	Wait() (*imapv2.SearchData, error)
}

type fetchMessage interface {
	Next() fetchItem
}

type fetchCommand interface {
	Next() fetchMessage
	Close() error
}

type fetchItem interface{}

type fetchBodySection interface {
	body() io.Reader
}

type fetchUID interface {
	value() uint32
}

type fetchFlags interface {
	values() []string
}

type sessionClient interface {
	Login(username, password string) loginCommand
	Select(mailbox string) selectCommand
	UIDSearch() searchCommand
	Fetch(uids []uint32, fullBody bool) fetchCommand
	Close() error
	Logout() error
}

type dialerFunc func(account config.AccountConfig) (sessionClient, error)

type RuntimeClient struct {
	dial dialerFunc
}

func NewRuntimeClient(dial dialerFunc) RuntimeClient {
	return RuntimeClient{dial: dial}
}

func NewDefaultRuntimeClient() RuntimeClient {
	return NewRuntimeClient(func(account config.AccountConfig) (sessionClient, error) {
		return wrapClientWithAccount(account)
	})
}

func (c RuntimeClient) ListRecent(ctx context.Context, account config.AccountConfig, mailbox string, limit int) ([]mail.Summary, error) {
	_ = ctx
	client, err := c.dial(account)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	defer client.Logout()

	if err := client.Login(account.Auth.Username, account.Auth.Password).Wait(); err != nil {
		return nil, err
	}
	if _, err := client.Select(mailbox).Wait(); err != nil {
		return nil, err
	}

	searchData, err := client.UIDSearch().Wait()
	if err != nil {
		return nil, err
	}
	uids := uidSlice(searchData)
	if len(uids) == 0 {
		return []mail.Summary{}, nil
	}
	if limit > 0 && len(uids) > limit {
		uids = uids[len(uids)-limit:]
	}

	messages, err := collectMessages(client.Fetch(uids, false))
	if err != nil {
		return nil, err
	}

	summaries := make([]mail.Summary, 0, len(messages))
	for _, message := range messages {
		detail, err := mail.ParseMessage(message.uid, message.flags, strings.NewReader(message.body))
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, detail.Summary)
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		return summaries[i].UID > summaries[j].UID
	})
	return summaries, nil
}

func (c RuntimeClient) GetByUID(ctx context.Context, account config.AccountConfig, mailbox string, uid uint32) (mail.Detail, error) {
	_ = ctx
	client, err := c.dial(account)
	if err != nil {
		return mail.Detail{}, err
	}
	defer client.Close()
	defer client.Logout()

	if err := client.Login(account.Auth.Username, account.Auth.Password).Wait(); err != nil {
		return mail.Detail{}, err
	}
	if _, err := client.Select(mailbox).Wait(); err != nil {
		return mail.Detail{}, err
	}

	messages, err := collectMessages(client.Fetch([]uint32{uid}, true))
	if err != nil {
		return mail.Detail{}, err
	}
	if len(messages) == 0 {
		return mail.Detail{}, fmt.Errorf("mail not found by uid %d", uid)
	}

	return mail.ParseMessage(messages[0].uid, messages[0].flags, strings.NewReader(messages[0].body))
}

type fetchedMessage struct {
	uid   uint32
	flags []string
	body  string
}

func collectMessages(command fetchCommand) ([]fetchedMessage, error) {
	defer command.Close()

	var messages []fetchedMessage
	for {
		message := command.Next()
		if message == nil {
			break
		}

		current := fetchedMessage{}
		for {
			item := message.Next()
			if item == nil {
				break
			}

			switch typed := item.(type) {
			case fetchUID:
				current.uid = typed.value()
			case fetchFlags:
				current.flags = typed.values()
			case fetchBodySection:
				body, err := io.ReadAll(typed.body())
				if err != nil {
					return nil, err
				}
				current.body = string(body)
			}
		}

		if current.uid != 0 {
			messages = append(messages, current)
		}
	}

	if err := command.Close(); err != nil {
		return nil, err
	}
	return messages, nil
}

func uidSlice(searchData *imapv2.SearchData) []uint32 {
	if searchData == nil || searchData.All == nil {
		return nil
	}

	uidSet, ok := searchData.All.(imapv2.UIDSet)
	if !ok {
		return nil
	}
	uids, ok := uidSet.Nums()
	if !ok {
		return nil
	}
	result := make([]uint32, 0, len(uids))
	for _, uid := range uids {
		result = append(result, uint32(uid))
	}
	return result
}

type wrappedClient struct {
	inner *imapclient.Client
}

func wrapClient(client *imapclient.Client) sessionClient {
	return wrappedClient{inner: client}
}

func wrapClientWithAccount(account config.AccountConfig) (sessionClient, error) {
	address := net.JoinHostPort(account.IMAP.Host, fmt.Sprintf("%d", account.IMAP.Port))
	options := &imapclient.Options{}
	if account.IMAP.TLS {
		options.TLSConfig = &tls.Config{ServerName: account.IMAP.Host}
		client, err := imapclient.DialTLS(address, options)
		if err != nil {
			return nil, err
		}
		return wrapClient(client), nil
	}

	client, err := imapclient.DialStartTLS(address, options)
	if err != nil {
		return nil, err
	}
	return wrapClient(client), nil
}

func (c wrappedClient) Login(username, password string) loginCommand {
	return c.inner.Login(username, password)
}

func (c wrappedClient) Select(mailbox string) selectCommand {
	return c.inner.Select(mailbox, nil)
}

func (c wrappedClient) UIDSearch() searchCommand {
	return c.inner.UIDSearch(&imapv2.SearchCriteria{UID: []imapv2.UIDSet{imapv2.UIDSetNum(1, 0)}}, nil)
}

func (c wrappedClient) Fetch(uids []uint32, _ bool) fetchCommand {
	uidSet := make([]imapv2.UID, 0, len(uids))
	for _, uid := range uids {
		uidSet = append(uidSet, imapv2.UID(uid))
	}
	bodySection := &imapv2.FetchItemBodySection{}
	return wrappedFetchCommand{inner: c.inner.Fetch(imapv2.UIDSetNum(uidSet...), &imapv2.FetchOptions{
		UID:         true,
		Flags:       true,
		BodySection: []*imapv2.FetchItemBodySection{bodySection},
	})}
}

func (c wrappedClient) Close() error {
	return c.inner.Close()
}

func (c wrappedClient) Logout() error {
	return c.inner.Logout().Wait()
}

type wrappedFetchCommand struct {
	inner *imapclient.FetchCommand
}

func (c wrappedFetchCommand) Next() fetchMessage {
	message := c.inner.Next()
	if message == nil {
		return nil
	}
	return wrappedFetchMessage{inner: message}
}

func (c wrappedFetchCommand) Close() error {
	return c.inner.Close()
}

type wrappedFetchMessage struct {
	inner *imapclient.FetchMessageData
}

func (m wrappedFetchMessage) Next() fetchItem {
	item := m.inner.Next()
	if item == nil {
		return nil
	}
	switch typed := item.(type) {
	case imapclient.FetchItemDataUID:
		return wrappedFetchUID{uid: uint32(typed.UID)}
	case imapclient.FetchItemDataFlags:
		flags := make([]string, 0, len(typed.Flags))
		for _, flag := range typed.Flags {
			flags = append(flags, string(flag))
		}
		return wrappedFetchFlags{flags: flags}
	case imapclient.FetchItemDataBodySection:
		return wrappedFetchBodySection{literal: typed.Literal}
	default:
		return nil
	}
}

type wrappedFetchBodySection struct {
	literal io.Reader
}

func (f wrappedFetchBodySection) body() io.Reader {
	return f.literal
}

type wrappedFetchUID struct {
	uid uint32
}

func (f wrappedFetchUID) value() uint32 {
	return f.uid
}

type wrappedFetchFlags struct {
	flags []string
}

func (f wrappedFetchFlags) values() []string {
	return append([]string(nil), f.flags...)
}

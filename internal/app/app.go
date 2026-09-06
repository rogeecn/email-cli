package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/rogeecn/email-cli/internal/config"
	"github.com/rogeecn/email-cli/internal/mail"
	"github.com/rogeecn/email-cli/internal/mailclaw"
	"github.com/rogeecn/email-cli/internal/output"
	"github.com/rogeecn/email-cli/internal/provider"
)

const (
	ModeList   = "list"
	ModeDetail = "detail"
)

type Loader interface {
	Load() (config.Config, error)
}

type ListResult struct {
	Summaries []mail.Summary
	Total     int
}

type MailService interface {
	ListRecent(ctx context.Context, account config.AccountConfig, mailbox string, limit int, offset int) (ListResult, error)
	GetByUID(ctx context.Context, account config.AccountConfig, mailbox string, uid uint32) (mail.Detail, error)
}

type Application struct {
	loader      Loader
	mailService MailService
}

type Result struct {
	Mode         string
	Format       string
	Account      string
	ListMetadata output.ListMetadata
	Summaries    []mail.Summary
	Detail       mail.Detail
	APIData      json.RawMessage
}

func New(loader Loader, mailService MailService) Application {
	return Application{loader: loader, mailService: mailService}
}

func (a Application) Run(ctx context.Context, options Options) (Result, error) {
	if options.Limit < 0 || options.Offset < 0 {
		return Result{}, errors.New("limit and offset must be non-negative")
	}
	cfg, err := a.loader.Load()
	if err != nil {
		return Result{}, err
	}

	accountName, account, err := config.ResolveAccount(cfg, options.Account)
	if err != nil {
		return Result{}, err
	}

	account, err = provider.Normalize(account)
	if err != nil {
		return Result{}, err
	}

	request := BuildRequest(options, AccountDefaults{
		Mailbox:  account.Defaults.Mailbox,
		PageSize: account.Defaults.PageSize,
		Format:   account.Defaults.Format,
	})

	if _, err := output.RenderAPI(json.RawMessage(`{}`), request.Format); err != nil {
		return Result{}, err
	}
	if request.Limit < 1 {
		return Result{}, errors.New("page size must be positive")
	}
	result := Result{Format: request.Format, Account: accountName}
	if account.Provider == "mailclaw" {
		if options.UID != 0 || options.Mailbox != "" {
			return Result{}, errors.New("MailClaw has no IMAP UID or mailbox; use --id or email-cli mailclaw <command>")
		}
		if request.Limit > 100 {
			return Result{}, errors.New("MailClaw page size must be 1..100")
		}
		endpoint := "/api/emails"
		params := url.Values{"limit": {strconv.Itoa(request.Limit)}, "offset": {strconv.Itoa(request.Offset)}}
		result.Mode = ModeList
		if options.ID != "" {
			endpoint, err = mailclaw.EmailPath(options.ID)
			if err != nil {
				return Result{}, err
			}
			params = nil
			result.Mode = ModeDetail
		}
		path, err := config.MailClawPath(account.MailClaw.Config)
		if err != nil {
			return Result{}, err
		}
		settings, err := config.ResolveMailClaw(path)
		if err != nil {
			return Result{}, err
		}
		client, err := mailclaw.New(settings.Host, settings.APIToken)
		if err != nil {
			return Result{}, err
		}
		result.APIData, err = client.Do(ctx, http.MethodGet, endpoint, params, nil)
		return result, err
	}
	if options.ID != "" {
		return Result{}, errors.New("--id is for MailClaw; use --uid for IMAP")
	}
	if request.DetailUID != 0 {
		detail, err := a.mailService.GetByUID(ctx, account, request.Mailbox, request.DetailUID)
		if err != nil {
			return Result{}, err
		}
		result.Mode = ModeDetail
		result.Detail = detail
		return result, nil
	}

	listResult, err := a.mailService.ListRecent(ctx, account, request.Mailbox, request.Limit, request.Offset)
	if err != nil {
		return Result{}, err
	}
	result.Mode = ModeList
	result.ListMetadata = output.ListMetadata{
		Total:  listResult.Total,
		Limit:  request.Limit,
		Offset: request.Offset,
	}
	result.Summaries = listResult.Summaries
	return result, nil
}

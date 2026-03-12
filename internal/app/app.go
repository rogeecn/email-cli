package app

import (
	"context"

	"github.com/rogeecn/email-cli/internal/config"
	"github.com/rogeecn/email-cli/internal/mail"
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
}

func New(loader Loader, mailService MailService) Application {
	return Application{loader: loader, mailService: mailService}
}

func (a Application) Run(ctx context.Context, options Options) (Result, error) {
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

	result := Result{Format: request.Format, Account: accountName}
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

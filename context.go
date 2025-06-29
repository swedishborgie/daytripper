package daytripper

import (
	"context"
	"github.com/swedishborgie/daytripper/har"
	"time"
)

type contextKey string

const (
	contextKeyStartPage contextKey = "start_page"
	contextKeyPage      contextKey = "page"
	contextKeyEndPage   contextKey = "end_page"
	contextKeyInclude   contextKey = "include"
)

func StartPage(ctx context.Context, id, title, comment string) context.Context {
	page := &har.Page{
		ID:              id,
		Title:           title,
		Comment:         comment,
		StartedDateTime: time.Now(),
		PageTimings:     &har.PageTimings{},
	}
	return Page(context.WithValue(ctx, contextKeyStartPage, page), id)
}

func Page(ctx context.Context, pageID string) context.Context {
	return context.WithValue(ctx, contextKeyPage, pageID)
}

func EndPage(ctx context.Context, pageID string) context.Context {
	return Page(context.WithValue(ctx, contextKeyEndPage, pageID), pageID)
}

func IncludeContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKeyInclude, true)
}

func pageFromCtx(ctx context.Context) string {
	pgInf := ctx.Value(contextKeyPage)
	if pgInf == nil {
		return ""
	}

	pageID, ok := ctx.Value(contextKeyPage).(string)
	if !ok {
		return ""
	}

	return pageID
}

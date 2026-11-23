//go:build wireinject

package app

import (
	"log/slog"

	"github.com/google/wire"
	"slate-kanban/internal/conf"
	"slate-kanban/internal/data"
	"slate-kanban/internal/web/api"
)

func WireApp(bc *conf.Bootstrap, log *slog.Logger) (Services, func(), error) {
	panic(wire.Build(data.ProviderSet, api.ProviderVersionSet, api.ProviderSet, wire.Struct(new(Services), "*")))
}

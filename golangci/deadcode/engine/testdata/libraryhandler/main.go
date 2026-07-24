package main

import (
	"context"
	"log/slog"
)

// Handler is invoked only by log/slog, never by this program.
type Handler struct{}

func (h *Handler) Enabled(context.Context, slog.Level) bool  { return true }
func (h *Handler) Handle(context.Context, slog.Record) error { return nil }
func (h *Handler) WithAttrs([]slog.Attr) slog.Handler        { return h }
func (h *Handler) WithGroup(string) slog.Handler             { return h }

func main() {
	slog.New(&Handler{}).Info("hello")
}

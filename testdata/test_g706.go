package main

import (
	"log/slog"
	"net/http"
	"strings"
)

type SafeString string

func clean1(s string) string     { return strings.ReplaceAll(s, "\n", "") }
func clean2(s string) SafeString { return SafeString(strings.ReplaceAll(s, "\n", "")) }
func handler(w http.ResponseWriter, r *http.Request) {
	slog.Info("req 1", "val", r.Header.Get("X"))
	slog.Info("req 2", "val", clean1(r.Header.Get("X")))
	slog.Info("req 3", "val", clean2(r.Header.Get("X")))
}

package logging

/*
ISC License

Copyright (c) 2026 Shane & Contributors

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
*/

import (
	"os"

	golog "github.com/Bugs5382/go-log"
	"github.com/rs/zerolog"
)

// Service is the service name attached to every log line.
const Service = "release-drafter-action"

var logger = zerolog.Nop()

// Defaults returns the LOG_LEVEL and LOG_FORMAT the action should use when
// the environment does not set them. Inside Actions the log is read in the
// run page, so it stays human-readable at info (debug when the run was
// re-run with debug logging). Anywhere else it is a local run: trace.
func Defaults(getenv func(string) string) (level, format string) {
	level, format = getenv("LOG_LEVEL"), getenv("LOG_FORMAT")
	if level == "" {
		switch {
		case getenv("GITHUB_ACTIONS") == "true" && getenv("RUNNER_DEBUG") == "1":
			level = "debug"
		case getenv("GITHUB_ACTIONS") == "true":
			level = "info"
		default:
			level = "trace"
		}
	}
	if format == "" {
		format = "console"
	}
	return level, format
}

// Init applies Defaults to the process environment and builds the logger.
// go-log reads LOG_LEVEL and LOG_FORMAT itself, so they are set before New.
func Init() zerolog.Logger {
	level, format := Defaults(os.Getenv)
	_ = os.Setenv("LOG_LEVEL", level)
	_ = os.Setenv("LOG_FORMAT", format)
	logger = golog.New(Service)
	return logger
}

// L returns the process logger. Before Init it discards everything, which
// keeps unit tests quiet.
func L() *zerolog.Logger { return &logger }

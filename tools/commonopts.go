package tools

import (
	"flag"
	"io"
	"log/slog"

	"github.com/Kuniwak/puml-parallel/slograw"
)

type CommonOptions struct {
	Help     bool
	Version  bool
	LogLevel slog.Level
}

var CommonOptionsHelp = &CommonOptions{Help: true}
var CommonOptionsVersion = &CommonOptions{Version: true}

func NewCommonOptionsDefault() *CommonOptions {
	return &CommonOptions{
		LogLevel: slog.LevelInfo,
	}
}

type CommonRawOptions struct {
	Help         bool
	ShortVersion bool
	Version      bool
	Silent       bool
	Debug        bool
}

func DeclareCommonOptions(flags *flag.FlagSet, options *CommonRawOptions) {
	flags.BoolVar(&options.ShortVersion, "v", false, "show version")
	flags.BoolVar(&options.Version, "version", false, "show version")
	flags.BoolVar(&options.Silent, "silent", false, "silent mode")
	flags.BoolVar(&options.Debug, "debug", false, "debug mode")
}

func ValidateCommonOptions(options *CommonRawOptions) (*CommonOptions, error) {
	if options.ShortVersion || options.Version {
		return &CommonOptions{Version: true}, nil
	}

	opts := NewCommonOptionsDefault()
	if options.Debug {
		opts.LogLevel = slog.LevelDebug
	} else if options.Silent {
		opts.LogLevel = slog.LevelError
	}

	return opts, nil
}

func NewLogger(logLevel slog.Level, w io.Writer) *slog.Logger {
	return slog.New(slograw.NewHandler(w, logLevel))
}

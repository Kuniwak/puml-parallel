package tools

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/slograw"
)

type CommonOptions struct {
	Help     bool
	Version  bool
	LogLevel slog.Level
}

// Debug reports whether debug mode is on, i.e. whether verbose, full-chain
// error output (and debug-level logging) is requested.
func (o *CommonOptions) Debug() bool { return o.LogLevel == slog.LevelDebug }

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

// ValidateArgsAsFilePath reads the single input of a tool from the file named
// by args, or from standard input when args is empty or the single argument is
// "-".
func ValidateArgsAsFilePath(args []string, inout *cli.ProcInout) ([]byte, error) {
	_, bs, err := ValidateArgsAsFileInput(args, inout)
	return bs, err
}

// ValidateArgsAsFileInput is ValidateArgsAsFilePath that also reports where the
// input came from: the path of the file it was read from, or "" for standard
// input. Tools that resolve paths relative to their input need the location.
func ValidateArgsAsFileInput(args []string, inout *cli.ProcInout) (string, []byte, error) {
	switch len(args) {
	case 0:
		bs, err := readStdin(inout)
		return "", bs, err

	case 1:
		file := args[0]
		if file == "-" {
			bs, err := readStdin(inout)
			return "", bs, err
		}

		bs, err := os.ReadFile(file)
		if err != nil {
			return "", nil, fmt.Errorf("cannot read file: %v", err)
		}
		return file, bs, nil

	default:
		return "", nil, fmt.Errorf("too many arguments")
	}
}

func readStdin(inout *cli.ProcInout) ([]byte, error) {
	bs, err := io.ReadAll(inout.Stdin)
	if err != nil {
		return nil, fmt.Errorf("cannot read from stdin: %v", err)
	}
	return bs, nil
}

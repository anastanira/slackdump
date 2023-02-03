package config

import (
	"errors"
	"fmt"
	"text/template"

	"github.com/rusq/slackdump/v2"
	"github.com/rusq/slackdump/v2/export"
	"github.com/rusq/slackdump/v2/internal/app/nametmpl"
	"github.com/rusq/slackdump/v2/internal/structures"
	"github.com/rusq/slackdump/v2/logger"
)

const (
	OutputTypeJSON = "json"
	OutputTypeText = "text"
)

const (
	FilenameTmplName = "fnt"
)

// ErrSkip should be returned if the [Producer] should skip the channel.
var ErrSkip = errors.New("skip")

// Params is the application config parameters.
type Params struct {
	ListFlags ListFlags

	Input  Input  // parameters of the input
	Output Output // " " output

	Oldest TimeValue // oldest time to dump conversations from
	Latest TimeValue // latest time to dump conversations to

	FilenameTemplate string

	ExportName  string            // export file or directory name.
	ExportType  export.ExportType // export type, see enum for available options.
	ExportToken string            // token that will be added to all exported files.

	Emoji EmojiParams

	SlackConfig slackdump.Config

	Log logger.Interface
}

type EmojiParams struct {
	Enabled     bool
	FailOnError bool
}

type Output struct {
	Filename string
	Format   string // output format
	Base     string // base directory or zip file
}

type Input struct {
	List *structures.EntityList // Include channels
}

var (
	ErrInvalidInput = errors.New("no valid input")
)

func (in *Input) IsValid() bool {
	return !in.List.IsEmpty()
}

// listProducer iterates over the input.List.Include, and calls fn for each
// entry.
func (in *Input) listProducer(fn func(string) error) error {
	if !in.List.HasIncludes() {
		return ErrInvalidInput
	}
	for _, entry := range in.List.Include {
		if err := fn(entry); err != nil {
			if errors.Is(err, ErrSkip) {
				continue
			}
			return err
		}
	}
	return nil
}

func (out Output) FormatValid() bool {
	return out.Format != "" && (out.Format == OutputTypeJSON ||
		out.Format == OutputTypeText)
}

func (out Output) IsText() bool {
	return out.Format == OutputTypeText
}

type ListFlags struct {
	Users    bool
	Channels bool
}

func (lf ListFlags) FlagsPresent() bool {
	return lf.Users || lf.Channels
}

var ErrNothingToDo = errors.New("no valid input and no list flags specified")

// Validate checks if the command line parameters have valid values.
func (p *Params) Validate() error {
	if p.ExportName != "" {
		// slack workspace export mode.
		return nil
	}

	if p.Emoji.Enabled {
		// emoji export mode
		if p.Output.Base == "" {
			return errors.New("emoji mode requires base directory")
		}
		return nil
	}

	if !p.Input.IsValid() && !p.ListFlags.FlagsPresent() {
		return ErrNothingToDo
	}

	// channels and users listings will be in the text format (if not specified otherwise)
	if p.Output.Format == "" {
		if p.ListFlags.FlagsPresent() {
			p.Output.Format = OutputTypeText
		} else {
			p.Output.Format = OutputTypeJSON
		}
	}

	if !p.ListFlags.FlagsPresent() && !p.Output.FormatValid() {
		return fmt.Errorf("invalid output type: %q, must use one of %v", p.Output.Format, []string{OutputTypeJSON, OutputTypeText})
	}

	// validate file naming template
	if _, err := p.CompileTemplates(); err != nil {
		return err
	}

	return nil
}

func (p *Params) CompileTemplates() (*template.Template, error) {
	return nametmpl.Compile(p.FilenameTemplate)
}

// Producer iterates over the list or reads the list from the file and calls
// fn for each entry.
func (in Input) Producer(fn func(string) error) error {
	if !in.IsValid() {
		return ErrInvalidInput
	}

	return in.listProducer(fn)
}

func (p *Params) Logger() logger.Interface {
	if p.Log == nil {
		return logger.Default
	}
	return p.Log
}

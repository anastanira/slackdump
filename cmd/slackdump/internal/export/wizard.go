package export

import (
	"context"

	"github.com/rusq/dlog"
	"github.com/rusq/slackdump/v2"
	"github.com/rusq/slackdump/v2/auth"
	"github.com/rusq/slackdump/v2/cmd/slackdump/internal/cfg"
	"github.com/rusq/slackdump/v2/cmd/slackdump/internal/golang/base"
	"github.com/rusq/slackdump/v2/export"
	"github.com/rusq/slackdump/v2/fsadapter"
	"github.com/rusq/slackdump/v2/internal/app/ui"
	"github.com/rusq/slackdump/v2/internal/app/ui/ask"
)

func wizExport(ctx context.Context, cmd *base.Command, args []string) error {
	options.Logger = dlog.FromContext(ctx)
	prov, err := auth.FromContext(ctx)
	if err != nil {
		return err
	}
	// ask for the list
	list, err := ask.ConversationList("Enter conversations to export (optional)?")
	if err != nil {
		return err
	}
	options.List = list

	// ask if user wants time range
	needRange, err := ui.Confirm("Do you want to specify the time range?", false)
	if err != nil {
		return err
	}
	var options export.Options
	if needRange {
		// ask for the time range
		if earliest, err := ui.Time("Earliest message"); err != nil {
			return err
		} else {
			options.Oldest = earliest
		}

		if latest, err := ui.Time("Latest message"); err != nil {
			return err
		} else {
			options.Latest = latest
		}
	}
	// ask for the type
	exportType, err := ask.ExportType()
	if err != nil {
		return err
	} else {
		options.Type = exportType
	}
	// ask for the save location
	baseLoc, err := ui.FileSelector("Output ZIP or Directory name", "Enter the name of the ZIP or directory to save the export to.")
	if err != nil {
		return err
	}
	fs, err := fsadapter.New(baseLoc)
	if err != nil {
		return err
	}

	sess, err := slackdump.NewWithOptions(ctx, prov, cfg.SlackOptions)
	if err != nil {
		return err
	}

	exp := export.New(sess, fs, options)

	// run export
	return exp.Run(ctx)
}

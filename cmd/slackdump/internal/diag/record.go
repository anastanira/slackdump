package diag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/rusq/slackdump/v2"
	"github.com/rusq/slackdump/v2/auth"
	"github.com/rusq/slackdump/v2/cmd/slackdump/internal/cfg"
	"github.com/rusq/slackdump/v2/cmd/slackdump/internal/golang/base"
	"github.com/rusq/slackdump/v2/internal/chunk"
)

var CmdRecord = &base.Command{
	UsageLine: "slackdump tools record",
	Short:     "chunk record commands",
	Commands:  []*base.Command{CmdRecordStream, CmdRecordState},
}

var CmdRecordStream = &base.Command{
	UsageLine: "slackdump tools record stream [options] <channel>",
	Short:     "dump slack data in a chunk record format",
	Long: `
# Record tool

Records the data from a channel in a chunk record format.

See also: slackdump tool obfuscate
`,
	FlagMask:    cfg.OmitBaseLocFlag | cfg.OmitDownloadFlag,
	PrintFlags:  true,
	RequireAuth: true,
}

var CmdRecordState = &base.Command{
	UsageLine:   "slackdump tools record state [options] <record_file.jsonl>",
	Short:       "print state of the record",
	FlagMask:    cfg.OmitAll,
	PrintFlags:  true,
	RequireAuth: false,
}

func init() {
	// break init cycle
	CmdRecordStream.Run = runRecord
}

var output = CmdRecordStream.Flag.String("output", "", "output file")

func runRecord(ctx context.Context, _ *base.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("missing channel argument")
	}

	prov, err := auth.FromContext(ctx)
	if err != nil {
		return err
	}
	sess, err := slackdump.New(ctx, prov)
	if err != nil {
		return err
	}

	var w io.Writer
	if *output == "" {
		w = os.Stdout
	} else {
		if f, err := os.Create(*output); err != nil {
			return err
		} else {
			defer f.Close()
			w = f
		}
	}

	rec := chunk.NewRecorder(w)
	for _, ch := range args {
		cfg.Log.Printf("streaming channel %q", ch)
		if err := sess.Stream().Conversations(ctx, rec, ch); err != nil {
			if err2 := rec.Close(); err2 != nil {
				return fmt.Errorf("error streaming channel %q: %w; error closing recorder: %v", ch, err, err2)
			}
			return err
		}
	}
	if err := rec.Close(); err != nil {
		return err
	}
	st, err := rec.State()
	if err != nil {
		return err
	}
	return st.Save(*output + ".state")
}

func init() {
	// break init cycle
	CmdRecordState.Run = runRecordState
}

func runRecordState(ctx context.Context, _ *base.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("missing record file argument")
	}
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()

	cf, err := chunk.FromReader(f)
	if err != nil {
		return err
	}
	state, err := cf.State()
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(state)
}
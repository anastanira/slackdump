package list

import (
	"context"
	"fmt"
	"runtime/trace"
	"time"

	"github.com/rusq/slackdump/v3"
	"github.com/rusq/slackdump/v3/cmd/slackdump/internal/cfg"
	"github.com/rusq/slackdump/v3/cmd/slackdump/internal/golang/base"
	"github.com/rusq/slackdump/v3/internal/cache"
	"github.com/rusq/slackdump/v3/logger"
	"github.com/rusq/slackdump/v3/types"
)

var CmdListChannels = &base.Command{
	Run:        listChannels,
	UsageLine:  "slackdump list channels [flags] [filename]",
	PrintFlags: true,
	FlagMask:   cfg.OmitDownloadFlag,
	Short:      "list workspace channels",
	Long: fmt.Sprintf(`
# List Channels Command

Lists all visible channels for the currently logged in user.  The list
includes all public and private channels, groups, and private messages (DMs),
including archived ones.

Please note that it may take a while to retrieve all channels, if your
workspace has lots of them.

The channels are cached, and the cache is valid for %s.  Use the -no-chan-cache
and -chan-cache-retention flags to control the cache behavior.
`+sectListFormat, chanFlags.cache.Retention),

	RequireAuth: true,
}

func init() {
	CmdListChannels.Wizard = wizChannels
}

type channelOptions struct {
	noResolve bool
	cache     cacheOpts
}

type cacheOpts struct {
	Disabled  bool
	Retention time.Duration
	Filename  string
}

var chanFlags = channelOptions{
	noResolve: false,
	cache: cacheOpts{
		Disabled:  false,
		Retention: 20 * time.Minute,
		Filename:  "channels.json",
	},
}

func init() {
	CmdListChannels.Flag.BoolVar(&chanFlags.cache.Disabled, "no-chan-cache", chanFlags.cache.Disabled, "disable channel cache")
	CmdListChannels.Flag.DurationVar(&chanFlags.cache.Retention, "chan-cache-retention", chanFlags.cache.Retention, "channel cache retention time.  After this time, the cache is considered stale and will be refreshed.")
	CmdListChannels.Flag.BoolVar(&chanFlags.noResolve, "no-resolve", chanFlags.noResolve, "do not resolve user IDs to names")
}

func listChannels(ctx context.Context, cmd *base.Command, args []string) error {
	if err := list(ctx, func(ctx context.Context, sess *slackdump.Session) (any, string, error) {
		ctx, task := trace.NewTask(ctx, "listChannels")
		defer task.End()

		var filename = makeFilename("channels", sess.Info().TeamID, ".json")
		if len(args) > 0 {
			filename = args[0]
		}
		teamID := sess.Info().TeamID
		cc, ok := maybeLoadChanCache(cfg.CacheDir(), teamID)
		if ok {
			// cache hit
			trace.Logf(ctx, "cache hit", "teamID=%s", teamID)
			return cc, filename, nil
		}
		// cache miss, load from API
		trace.Logf(ctx, "cache miss", "teamID=%s", teamID)
		cc, err := sess.GetChannels(ctx)
		if err != nil {
			return nil, "", fmt.Errorf("error getting channels: %w", err)
		}
		if err := saveCache(cfg.CacheDir(), teamID, cc); err != nil {
			// warn, but don't fail
			logger.FromContext(ctx).Printf("failed to save cache: %v", err)
		}
		return cc, filename, nil
	}); err != nil {
		return err
	}

	return nil
}

func maybeLoadChanCache(cacheDir string, teamID string) (types.Channels, bool) {
	if chanFlags.cache.Disabled {
		// channel cache disabled
		return nil, false
	}
	m, err := cache.NewManager(cacheDir)
	if err != nil {
		return nil, false
	}
	cc, err := m.LoadChannels(teamID, chanFlags.cache.Retention)
	if err != nil {
		return nil, false
	}
	return cc, true
}

func saveCache(cacheDir, teamID string, cc types.Channels) error {
	m, err := cache.NewManager(cacheDir)
	if err != nil {
		return err
	}
	return m.CacheChannels(teamID, cc)
}

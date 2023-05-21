package chunk

import (
	"fmt"
	"strings"

	"github.com/rusq/slackdump/v2/internal/structures"
	"github.com/slack-go/slack"
)

// ChunkType is the type of chunk that was recorded.  There are three types:
// messages, thread messages, and files.
type ChunkType uint8

//go:generate stringer -type=ChunkType -trimprefix=C
const (
	CMessages ChunkType = iota
	CThreadMessages
	CFiles
	CUsers
	CChannels
	CChannelInfo
	CWorkspaceInfo
	CChannelUsers
	CStarredItems
	CBookmarks
)

var ErrUnsupChunkType = fmt.Errorf("unsupported chunk type")

// Chunk is a single chunk that was recorded.  It contains the type of chunk,
// the timestamp of the chunk, the channel ID, and the number of messages or
// files that were recorded.
type Chunk struct {
	// header
	Type      ChunkType `json:"t"`
	Timestamp int64     `json:"ts"`
	ChannelID string    `json:"id,omitempty"`
	Count     int       `json:"n,omitempty"` // number of messages or files

	// ThreadTS is populated if the chunk contains thread related data.  It
	// is the timestamp of the thread.
	ThreadTS string `json:"r,omitempty"`
	// IsLast is set to true if this is the last chunk for the channel or
	// thread. Populated by Messages and ThreadMessages methods.
	IsLast bool `json:"l,omitempty"`
	// Number of threads in the message chunk.  Populated by Messages method.
	NumThreads int `json:"nt,omitempty"`

	// Channel contains the channel information.  It may not be immediately
	// followed by messages from the channel.  Populated by ChannelInfo and
	// Files methods.
	Channel *slack.Channel `json:"ci,omitempty"`

	ChannelUsers []string `json:"cu,omitempty"` // Populated by ChannelUsers

	// Parent is populated in case the chunk is a thread, or a file. Populated
	// by ThreadMessages and Files methods.
	Parent *slack.Message `json:"p,omitempty"`
	// Messages contains a chunk of messages as returned by the API. Populated
	// by Messages and ThreadMessages methods.
	Messages []slack.Message `json:"m,omitempty"`
	// Files contains a chunk of files as returned by the API. Populated by
	// Files method.
	Files []slack.File `json:"f,omitempty"`

	// Users contains a chunk of users as returned by the API. Populated by
	// Users method.
	Users []slack.User `json:"u,omitempty"` // Populated by Users
	// Channels contains a chunk of channels as returned by the API. Populated
	// by Channels method.
	Channels []slack.Channel `json:"ch,omitempty"` // Populated by Channels
	// WorkspaceInfo contains the workspace information as returned by the
	// API.  Populated by WorkspaceInfo.
	WorkspaceInfo *slack.AuthTestResponse `json:"w,omitempty"`
	//
	StarredItems []slack.StarredItem `json:"st,omitempty"` // Populated by StarredItems
	//
	Bookmarks []slack.Bookmark `json:"b,omitempty"` // Populated by Bookmarks
}

// GroupID is a unique ID for a chunk group.  It is used to group chunks of
// the same type together for indexing purposes.  It may or may not be equal
// to the Slack ID of the entity.
type GroupID string

const (
	userChunkID    GroupID = "lusr"
	channelChunkID GroupID = "lch"
	starredChunkID GroupID = "ls"
	wspInfoChunkID GroupID = "iw"

	threadPrefix    = "t"
	filePrefix      = "f"
	chanInfoPrefix  = "ic"
	bookmarkPrefix  = "lb"
	chanUsersPrefix = "lcu"
)

// Chunk ID categories
const (
	catFile = 'f'
	catInfo = 'i'
	catList = 'l'
)

// ID returns a Group ID for the chunk.
func (c *Chunk) ID() GroupID {
	switch c.Type {
	case CMessages:
		return GroupID(c.ChannelID)
	case CThreadMessages:
		return threadID(c.ChannelID, c.Parent.ThreadTimestamp)
	case CFiles:
		return id(filePrefix, c.ChannelID, c.Parent.Timestamp)
	case CChannelInfo:
		return channelInfoID(c.ChannelID)
	case CChannelUsers:
		return channelUsersID(c.ChannelID)
	case CUsers:
		return userChunkID // static, one team per chunk file
	case CChannels:
		return channelChunkID // static
	case CWorkspaceInfo:
		return wspInfoChunkID // static
	case CStarredItems:
		return starredChunkID // static
	case CBookmarks:
		return id(bookmarkPrefix, c.ChannelID)
	}
	return GroupID(fmt.Sprintf("<unknown:%s>", c.Type))
}

func id(prefix string, ids ...string) GroupID {
	return GroupID(prefix + strings.Join(ids, ":"))
}

func threadID(channelID, threadTS string) GroupID {
	return id(threadPrefix, channelID, threadTS)
}

func channelInfoID(channelID string) GroupID {
	return id(chanInfoPrefix, channelID)
}

func channelUsersID(channelID string) GroupID {
	return id(chanUsersPrefix, channelID)
}

func (c *Chunk) String() string {
	return fmt.Sprintf("%s: %s", c.Type, c.ID())
}

// Timestamps returns the timestamps of the messages in the chunk.  For files
// and other types of chunks, it returns ErrUnsupChunkType.
func (c *Chunk) Timestamps() ([]int64, error) {
	switch c.Type {
	case CMessages, CThreadMessages:
		return c.messageTimestamps()
	default:
		return nil, ErrUnsupChunkType
	}
	// unreachable
}

func (c *Chunk) messageTimestamps() ([]int64, error) {
	ts := make([]int64, len(c.Messages))
	for i := range c.Messages {
		iTS, err := structures.TS2int(c.Messages[i].Timestamp)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp: %q: %w", c.Messages[i].Timestamp, err)
		}
		ts[i] = iTS
	}
	return ts, nil
}

// isInfo returns true, if the chunk is an info chunk.
func (g GroupID) isInfo() bool {
	return g[0] == catInfo
}

// isList returns true, if the chunk is a list chunk.
func (g GroupID) isList() bool {
	return g[0] == catList
}

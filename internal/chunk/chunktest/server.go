// Package chunktest provides a test server for testing the chunk package.
package chunktest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"runtime/trace"
	"strconv"

	"github.com/slack-go/slack"

	"github.com/rusq/slackdump/v2/internal/chunk"
)

// Server is a test server for testing the chunk package.
type Server struct {
	*httptest.Server
	p *chunk.Player
}

// NewServer returns a new Server.
func NewServer(rs io.ReadSeeker) *Server {
	p, err := chunk.NewPlayer(rs)
	if err != nil {
		panic(err)
	}
	return &Server{
		Server: httptest.NewServer(router(p)),
		p:      p,
	}
}

// Close closes the server.
func (s *Server) Close() {
	s.Server.Close()
}

type GetConversationRepliesResponse struct {
	slack.SlackResponse
	HasMore          bool             `json:"has_more"`
	ResponseMetaData responseMetaData `json:"response_metadata"`
	Messages         []slack.Message  `json:"messages"`
}

type responseMetaData struct {
	NextCursor string `json:"next_cursor"`
}

func router(p *chunk.Player) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/conversations.info", handleConversationsInfo(p))
	mux.HandleFunc("/api/conversations.history", handleConversationsHistory(p))
	mux.HandleFunc("/api/conversations.replies", handleConversationsReplies(p))
	mux.HandleFunc("/api/conversations.list", handleConversationsList(p))
	mux.HandleFunc("/api/users.list", handleUsersList(p))
	return mux
}

func handleConversationsHistory(p *chunk.Player) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, task := trace.NewTask(r.Context(), "conversation.history")
		defer task.End()

		channel := r.FormValue("channel")
		if channel == "" {
			http.NotFound(w, r)
			return
		}
		log.Printf("channel: %s", channel)

		msg, err := p.Messages(channel)
		if err != nil {
			if errors.Is(err, chunk.ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			if errors.Is(err, io.EOF) {
				if err := json.NewEncoder(w).Encode(slack.GetConversationHistoryResponse{
					HasMore: false,
					SlackResponse: slack.SlackResponse{
						Ok: true,
					},
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			log.Printf("error processing messages: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp := slack.GetConversationHistoryResponse{
			HasMore:          p.HasMoreMessages(channel),
			Messages:         msg,
			ResponseMetaData: responseMetaData{NextCursor: strconv.FormatInt(p.Offset(), 10)},
			SlackResponse: slack.SlackResponse{
				Ok: true,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("error encoding channel.history response: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func handleConversationsReplies(p *chunk.Player) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, task := trace.NewTask(r.Context(), "conversation.replies")
		defer task.End()

		timestamp := r.FormValue("ts")
		channel := r.FormValue("channel")
		log.Printf("channel: %s, ts: %s", channel, timestamp)

		if timestamp == "" {
			http.Error(w, "ts is required", http.StatusBadRequest)
			return
		}

		slackResp := slack.SlackResponse{
			Ok: true,
		}
		msg, err := p.Thread(channel, timestamp)
		if err != nil {
			slackResp.Ok = false
			if errors.Is(err, io.EOF) {
				slackResp.Error = fmt.Sprintf("thread_not_found[%s:%s]", channel, timestamp)
			} else {
				slackResp.Error = err.Error()
			}
			log.Printf("error processing thread: %s", err)
		}
		resp := GetConversationRepliesResponse{
			HasMore:          p.HasMoreThreads(channel, timestamp),
			Messages:         msg,
			ResponseMetaData: responseMetaData{strconv.FormatInt(p.Offset(), 10)}, // adding offset for the ease of debugging.
			SlackResponse:    slackResp,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("error encoding conversation.replies response: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

type channelResponseFull struct {
	Channel      slack.Channel `json:"channel"`
	Purpose      string        `json:"purpose"`
	Topic        string        `json:"topic"`
	NotInChannel bool          `json:"not_in_channel"`
	slack.History
	slack.SlackResponse
	Metadata slack.ResponseMetadata `json:"response_metadata"`
}

func handleConversationsInfo(p *chunk.Player) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, task := trace.NewTask(r.Context(), "conversation.info")
		defer task.End()

		channel := r.FormValue("channel")
		if channel == "" {
			http.Error(w, "channel is required", http.StatusBadRequest)
			return
		}
		log.Printf("channel: %s", channel)
		ci, err := p.ChannelInfo(channel)
		if err != nil {
			if errors.Is(err, chunk.ErrNotFound) {
				log.Printf("conversationInfo: not found: (%q) %v", channel, err)
				http.NotFound(w, r)
				return
			}
			log.Printf("conversationInfo: error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := channelResponseFull{
			SlackResponse: slack.SlackResponse{
				Ok: true,
			},
			Channel: *ci,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("error encoding channel.info response: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

type channelResponse struct {
	Channels []slack.Channel `json:"channels"`
	slack.SlackResponse
	Metadata slack.ResponseMetadata `json:"response_metadata"`
}

func handleConversationsList(p *chunk.Player) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, task := trace.NewTask(r.Context(), "conversation.list")
		defer task.End()

		c, err := p.Channels()
		sr := slack.SlackResponse{
			Ok: true,
			ResponseMetadata: slack.ResponseMetadata{
				Cursor: "moar",
			},
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				sr.Ok = false
				sr.ResponseMetadata.Cursor = ""
			} else {
				log.Printf("error processing conversations.list: %s", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		resp := channelResponse{
			Channels:      c,
			SlackResponse: sr,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("error encoding channel.list response: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

type userResponseFull struct {
	Users   []slack.User `json:"users,omitempty"`
	User    slack.User   `json:"user,omitempty"`
	Members []slack.User `json:"members"`
	slack.SlackResponse
	slack.UserPresence
	Metadata slack.ResponseMetadata `json:"response_metadata"`
}

func handleUsersList(p *chunk.Player) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, task := trace.NewTask(r.Context(), "users.list")
		defer task.End()

		sr := slack.SlackResponse{
			Ok: true,
		}
		u, err := p.Users()
		if err != nil {
			if errors.Is(err, io.EOF) {
				sr.Ok = false
				sr.Error = "pagination complete"
			} else {
				log.Printf("error processing users.list: %s", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		resp := userResponseFull{
			Users:         u,
			SlackResponse: sr,
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("error encoding users.list response: %s", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
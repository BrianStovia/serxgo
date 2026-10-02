package tiktoklive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	golive "github.com/PirateTok/live-go"
	"github.com/PirateTok/live-go/events"
	"github.com/PirateTok/live-go/proto"
)

// ChatUser represents a participant in the live stream chat
type ChatUser struct {
	ID       string `json:"id,omitempty"`
	Nickname string `json:"nickname"`
	Username string `json:"username"`
	Avatar   string `json:"avatar"`
}

// ChatEvent represents a payload sent over SSE
type ChatEvent struct {
	Type      string      `json:"type"` // "status", "chat", "gift", "like", "join", "end", "heartbeat"
	RoomID    string      `json:"room_id,omitempty"`
	User      *ChatUser   `json:"user,omitempty"`
	Message   string      `json:"message,omitempty"`
	GiftName  string      `json:"gift_name,omitempty"`
	GiftCount int32       `json:"gift_count,omitempty"`
	GiftIcon  string      `json:"gift_icon,omitempty"`
	Diamonds  int32       `json:"diamonds,omitempty"`
	LikeCount int64       `json:"like_count,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// ServeChatSSE handles Server-Sent Events for streaming real-time live comments
func ServeChatSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	rawUser := strings.TrimSpace(r.URL.Query().Get("user"))
	if rawUser == "" {
		rawUser = strings.TrimSpace(r.URL.Query().Get("u"))
	}
	username := CleanUsername(rawUser)
	if username == "" {
		http.Error(w, "username parameter is required", http.StatusBadRequest)
		return
	}

	// Set SSE Headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sendSSE := func(eventType string, data interface{}) bool {
		payload, err := json.Marshal(data)
		if err != nil {
			return true
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, payload); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	// Send initial connecting status
	sendSSE("status", ChatEvent{
		Type:      "status",
		Message:   fmt.Sprintf("Menghubungkan ke live chat @%s...", username),
		Timestamp: time.Now().Unix(),
	})

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	client := golive.NewClient(username).
		Timeout(15 * time.Second).
		StaleTimeout(30 * time.Second).
		UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")

	eventsChan, err := client.Connect(ctx)
	if err != nil {
		sendSSE("status", ChatEvent{
			Type:      "error",
			Message:   fmt.Sprintf("Tidak dapat terhubung ke live chat: %v", err),
			Timestamp: time.Now().Unix(),
		})
		return
	}

	// Heartbeat ticker to keep the HTTP connection alive
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	var msgCounter int64

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// Ping heartbeat comment
			if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()

		case ev, ok := <-eventsChan:
			if !ok {
				sendSSE("status", ChatEvent{
					Type:      "disconnected",
					Message:   "Koneksi live chat terputus",
					Timestamp: time.Now().Unix(),
				})
				return
			}

			atomic.AddInt64(&msgCounter, 1)

			switch ev.Type {
			case events.EventConnected:
				sendSSE("connected", ChatEvent{
					Type:      "connected",
					RoomID:    ev.RoomID,
					Message:   fmt.Sprintf("Berhasil terhubung ke live chat room %s", ev.RoomID),
					Timestamp: time.Now().Unix(),
				})

			case events.EventChat:
				if chat, ok := ev.Data.(*proto.WebcastChatMessage); ok && chat != nil {
					user := extractUser(chat.User)
					if !sendSSE("chat", ChatEvent{
						Type:      "chat",
						User:      user,
						Message:   chat.Content,
						Timestamp: time.Now().Unix(),
					}) {
						return
					}
				}

			case events.EventGift:
				if gift, ok := ev.Data.(*proto.WebcastGiftMessage); ok && gift != nil {
					user := extractUser(gift.User)
					gName := "Gift"
					var icon string
					var diamonds int32
					if gift.Gift != nil {
						gName = gift.Gift.Name
						diamonds = gift.Gift.DiamondCount
						if gift.Gift.Image != nil && len(gift.Gift.Image.UrlList) > 0 {
							icon = gift.Gift.Image.UrlList[0]
						}
					}
					repeat := gift.RepeatCount
					if repeat <= 0 {
						repeat = 1
					}
					if !sendSSE("gift", ChatEvent{
						Type:      "gift",
						User:      user,
						GiftName:  gName,
						GiftCount: repeat,
						GiftIcon:  icon,
						Diamonds:  diamonds,
						Timestamp: time.Now().Unix(),
					}) {
						return
					}
				}

			case events.EventLike:
				if like, ok := ev.Data.(*proto.WebcastLikeMessage); ok && like != nil {
					user := extractUser(like.User)
					sendSSE("like", ChatEvent{
						Type:      "like",
						User:      user,
						LikeCount: int64(like.Count),
						Timestamp: time.Now().Unix(),
					})
				}

			case events.EventJoin, events.EventMember:
				if member, ok := ev.Data.(*proto.WebcastMemberMessage); ok && member != nil {
					user := extractUser(member.User)
					// Sample joins so we don't flood the SSE pipe if thousands join per second
					if atomic.LoadInt64(&msgCounter)%3 == 0 {
						sendSSE("join", ChatEvent{
							Type:      "join",
							User:      user,
							Message:   "bergabung ke siaran",
							Timestamp: time.Now().Unix(),
						})
					}
				}

			case events.EventLiveEnded:
				sendSSE("end", ChatEvent{
					Type:      "end",
					Message:   "Siaran TikTok Live ini telah berakhir",
					Timestamp: time.Now().Unix(),
				})
				return
			}
		}
	}
}

func extractUser(u *proto.User) *ChatUser {
	if u == nil {
		return &ChatUser{
			Nickname: "Pengguna",
			Username: "user",
		}
	}
	nick := strings.TrimSpace(u.Nickname)
	if nick == "" {
		nick = strings.TrimSpace(u.DisplayId)
	}
	if nick == "" {
		nick = strings.TrimSpace(u.UniqueId)
	}
	if nick == "" {
		nick = "Pengguna TikTok"
	}

	unique := strings.TrimSpace(u.UniqueId)
	if unique == "" {
		unique = strings.TrimSpace(u.DisplayId)
	}

	var avatar string
	if u.AvatarThumb != nil && len(u.AvatarThumb.UrlList) > 0 {
		avatar = u.AvatarThumb.UrlList[0]
	} else if u.AvatarMedium != nil && len(u.AvatarMedium.UrlList) > 0 {
		avatar = u.AvatarMedium.UrlList[0]
	} else if u.AvatarLarge != nil && len(u.AvatarLarge.UrlList) > 0 {
		avatar = u.AvatarLarge.UrlList[0]
	}

	return &ChatUser{
		ID:       fmt.Sprintf("%d", u.Id),
		Nickname: nick,
		Username: unique,
		Avatar:   avatar,
	}
}

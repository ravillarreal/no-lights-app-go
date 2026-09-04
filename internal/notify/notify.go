// Package notify sends Telegram messages to nearby users.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ravillarreal/no-lights-app-go/internal/store"
)

// Store is the subset of *store.Store that notify needs.
type Store interface {
	NearbyUsers(ctx context.Context, lon, lat, radiusM float64, eventType string, cooldownMin int) ([]store.NearbyUser, error)
	LogNotification(ctx context.Context, userID int64, eventType string) error
}

type Client struct {
	token  string
	store  Store
	radius float64 // km
	client *http.Client
}

func New(token string, store Store, radiusKM float64) *Client {
	return &Client{
		token:  token,
		store:  store,
		radius: radiusKM,
		client: &http.Client{Timeout: 8 * time.Second},
	}
}

// NotifyNearby looks up users within radius of (lon, lat) and sends them a
// message. tieneLuz=false means "outage", true means "restored".
func (c *Client) NotifyNearby(ctx context.Context, lon, lat float64, tieneLuz bool) {
	if c.store == nil || c.token == "" {
		return
	}

	eventType := "outage"
	base := "⚡ <b>Se fue la luz</b>"
	if tieneLuz {
		eventType = "restored"
		base = "💡 <b>¡Llegó la luz!</b>"
	}

	radiusM := c.radius * 1000
	users, err := c.store.NearbyUsers(ctx, lon, lat, radiusM, eventType, 30)
	if err != nil {
		log.Printf("notify: nearby users query failed: %v", err)
		return
	}
	if len(users) == 0 {
		return
	}

	log.Printf("notify: sending to %d user(s) [%s]", len(users), eventType)
	for _, u := range users {
		msg := fmt.Sprintf("%s\n📍 Cerca de: <b>%s</b>", base, u.Labels)
		if c.send(ctx, u.ChatID, msg) {
			_ = c.store.LogNotification(ctx, u.TelegramUserID, eventType)
		}
	}
}

func (c *Client) send(ctx context.Context, chatID int64, text string) bool {
	payload, _ := json.Marshal(map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.token),
		bytes.NewReader(payload))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		log.Printf("notify: telegram send failed: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("notify: telegram rejected chat %d: %d", chatID, resp.StatusCode)
		return false
	}
	return true
}

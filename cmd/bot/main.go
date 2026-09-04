// Command bot is the Telegram bot: users register monitored locations and
// receive nearby outage/restoration notifications.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravillarreal/no-lights-app-go/internal/store"
)

// Conversation states for /add_location.
const (
	stateIdle = iota
	stateWaitingLocation
	stateWaitingLabel
)

type userState struct {
	state      int
	pendingLon float64
	pendingLat float64
}

type bot struct {
	api   *tgbotapi.BotAPI
	store *store.Store
	mu    sync.Mutex
	users map[int64]*userState
}

func main() {
	token := os.Getenv("TELEGRAM_TOKEN")
	dbURL := os.Getenv("DATABASE_URL")
	if token == "" {
		log.Fatal("TELEGRAM_TOKEN is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("cannot create pool: %v", err)
	}
	defer pool.Close()

	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("bot init: %v", err)
	}

	b := &bot{
		api:   api,
		store: store.New(pool),
		users: make(map[int64]*userState),
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	for update := range api.GetUpdatesChan(u) {
		b.handle(ctx, update)
	}
}

func (b *bot) handle(ctx context.Context, update tgbotapi.Update) {
	if update.CallbackQuery != nil {
		b.handleDeleteCallback(ctx, update.CallbackQuery)
		return
	}
	if update.Message == nil {
		return
	}
	msg := update.Message

	userID := msg.From.ID
	b.mu.Lock()
	st := b.users[userID]
	if st == nil {
		st = &userState{state: stateIdle}
		b.users[userID] = st
	}
	b.mu.Unlock()

	switch st.state {
	case stateWaitingLocation:
		if msg.Location != nil {
			b.receiveLocation(ctx, msg, st)
		} else {
			b.reply(msg.Chat.ID, "Envíame una ubicación usando el botón o el mapa de Telegram.")
		}
		return
	case stateWaitingLabel:
		if msg.IsCommand() && msg.Command() == "cancel" {
			b.cancel(msg.Chat.ID, st)
		} else if msg.IsCommand() && msg.Command() == "skip" {
			b.saveLocation(ctx, msg, st, "Mi ubicación")
		} else {
			label := strings.TrimSpace(msg.Text)
			if label == "" {
				label = "Mi ubicación"
			}
			if len(label) > 80 {
				label = label[:80]
			}
			b.saveLocation(ctx, msg, st, label)
		}
		return
	}

	switch msg.Command() {
	case "start":
		b.start(msg.Chat.ID)
	case "add_location":
		b.addLocation(msg.Chat.ID, st)
	case "mis_ubicaciones":
		b.listLocations(ctx, msg, userID)
	case "borrar_ubicacion":
		b.deleteMenu(ctx, msg, userID)
	case "cancel":
		b.cancel(msg.Chat.ID, st)
	default:
		b.unknown(msg.Chat.ID, msg.Text)
	}
}

func (b *bot) start(chatID int64) {
	b.replyHTML(chatID, "👋 <b>Bot de Cortes de Luz</b>\n\n"+
		"Te aviso cuando se vaya o llegue la luz cerca de tus ubicaciones guardadas.\n\n"+
		"Comandos:\n"+
		"  /add_location — Guardar una ubicación\n"+
		"  /mis_ubicaciones — Ver tus ubicaciones\n"+
		"  /borrar_ubicacion — Eliminar una ubicación\n"+
		"  /cancel — Cancelar operación")
}

func (b *bot) addLocation(chatID int64, st *userState) {
	st.state = stateWaitingLocation
	btn := tgbotapi.NewKeyboardButtonLocation("📍 Usar mi ubicación actual")
	kb := tgbotapi.NewReplyKeyboard([]tgbotapi.KeyboardButton{btn})
	msg := tgbotapi.NewMessage(chatID, "Envíame la ubicación que quieres monitorear.\n"+
		"Puedes tocar el botón para usar tu posición actual, o compartir cualquier punto del mapa.")
	msg.ReplyMarkup = kb
	b.send(msg)
}

func (b *bot) receiveLocation(_ context.Context, msg *tgbotapi.Message, st *userState) {
	st.pendingLon = msg.Location.Longitude
	st.pendingLat = msg.Location.Latitude
	st.state = stateWaitingLabel
	b.reply(msg.Chat.ID, "¿Cómo quieres llamar a esta ubicación? (ej: Casa, Trabajo, Hospital)\n"+
		"O envía /skip para guardarla sin nombre.")
}

func (b *bot) saveLocation(ctx context.Context, msg *tgbotapi.Message, st *userState, label string) {
	id, err := b.store.SaveLocation(ctx, msg.From.ID, msg.Chat.ID, firstOr(msg.From.FirstName, "Usuario"), label, st.pendingLon, st.pendingLat)
	st.state = stateIdle
	if err != nil {
		log.Printf("save location: %v", err)
		b.reply(msg.Chat.ID, "Error guardando la ubicación, inténtalo de nuevo.")
		return
	}
	b.replyHTML(msg.Chat.ID, fmt.Sprintf("✅ <b>%s</b> guardada (#%d).\nTe notificaré cuando se vaya o llegue la luz cerca de ahí.", label, id))
}

func (b *bot) listLocations(ctx context.Context, msg *tgbotapi.Message, userID int64) {
	locs, err := b.store.UserLocations(ctx, userID, 10)
	if err != nil {
		log.Printf("list locations: %v", err)
		b.reply(msg.Chat.ID, "Error consultando tus ubicaciones.")
		return
	}
	if len(locs) == 0 {
		b.reply(msg.Chat.ID, "No tienes ubicaciones guardadas.\nUsa /add_location para agregar una.")
		return
	}
	lines := []string{fmt.Sprintf("📍 <b>Tus ubicaciones (%d):</b>", len(locs))}
	for _, l := range locs {
		lines = append(lines, fmt.Sprintf("• <b>%s</b> — %.5f, %.5f <i>(#%d)</i>", l.Label, l.Lat, l.Lon, l.ID))
	}
	lines = append(lines, "\nUsa /borrar_ubicacion para eliminar alguna.")
	b.replyHTML(msg.Chat.ID, strings.Join(lines, "\n"))
}

func (b *bot) deleteMenu(ctx context.Context, msg *tgbotapi.Message, userID int64) {
	locs, err := b.store.UserLocations(ctx, userID, 0)
	if err != nil {
		log.Printf("delete menu: %v", err)
		b.reply(msg.Chat.ID, "Error consultando tus ubicaciones.")
		return
	}
	if len(locs) == 0 {
		b.reply(msg.Chat.ID, "No tienes ubicaciones guardadas.\nUsa /add_location para agregar una.")
		return
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, l := range locs {
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("🗑 %s  (%.4f, %.4f)", l.Label, l.Lat, l.Lon),
				fmt.Sprintf("del:%d", l.ID),
			),
		})
	}
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("Cancelar", "del:cancel"),
	})

	out := tgbotapi.NewMessage(msg.Chat.ID, "¿Qué ubicación quieres eliminar?")
	out.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	b.send(out)
}

func (b *bot) handleDeleteCallback(ctx context.Context, q *tgbotapi.CallbackQuery) {
	if _, err := b.api.Request(tgbotapi.NewCallback(q.ID, "")); err != nil {
		log.Printf("callback answer: %v", err)
	}

	chatID := q.Message.Chat.ID
	if q.Data == "del:cancel" {
		edit := tgbotapi.NewEditMessageText(chatID, q.Message.MessageID, "Cancelado.")
		b.send(edit)
		return
	}

	id, err := strconv.Atoi(strings.TrimPrefix(q.Data, "del:"))
	if err != nil {
		return
	}
	label, ok, err := b.store.DeleteLocation(ctx, q.From.ID, id)
	if err != nil {
		log.Printf("delete location: %v", err)
		return
	}
	if !ok {
		edit := tgbotapi.NewEditMessageText(chatID, q.Message.MessageID, "No se encontró la ubicación.")
		b.send(edit)
		return
	}
	edit := tgbotapi.NewEditMessageText(chatID, q.Message.MessageID, fmt.Sprintf("✅ Ubicación <b>%s</b> eliminada.", label))
	edit.ParseMode = "HTML"
	b.send(edit)
}

func (b *bot) cancel(chatID int64, st *userState) {
	st.state = stateIdle
	st.pendingLon, st.pendingLat = 0, 0
	b.reply(chatID, "Cancelado.")
}

func (b *bot) unknown(chatID int64, text string) {
	b.replyHTML(chatID, fmt.Sprintf("❓ Comando <code>%s</code> no reconocido.\n\n"+
		"Comandos disponibles:\n"+
		"  /start — Bienvenida e instrucciones\n"+
		"  /add_location — Guardar una ubicación\n"+
		"  /mis_ubicaciones — Ver tus ubicaciones\n"+
		"  /borrar_ubicacion — Eliminar una ubicación\n"+
		"  /cancel — Cancelar operación actual", text))
}

func (b *bot) reply(chatID int64, text string) {
	b.send(tgbotapi.NewMessage(chatID, text))
}

func (b *bot) replyHTML(chatID int64, text string) {
	m := tgbotapi.NewMessage(chatID, text)
	m.ParseMode = "HTML"
	b.send(m)
}

func (b *bot) send(c tgbotapi.Chattable) {
	if _, err := b.api.Send(c); err != nil {
		log.Printf("telegram send: %v", err)
	}
}

func firstOr(s, def string) string {
	if s != "" {
		return s
	}
	return def
}

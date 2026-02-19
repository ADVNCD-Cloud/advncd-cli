package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type app struct {
	db                     *DB
	bot                    *tgbotapi.BotAPI
	planID                 string
	adminTGID              int64
	tz                     string
	debugAlwaysShowActions bool
	debugAllowAnyDay       bool

	webhookSecret string
	jobsSecret    string
	jobsTZ        string
}

func main() {
	_ = godotenv.Load()

	// TEMP/SAFE diagnostics (no values, just presence)
	log.Printf("env present: TG_BOT_TOKEN=%t DATABASE_URL=%t PLAN_ID=%t WEBHOOK_SECRET=%t ADMIN_TG_ID=%t",
		strings.TrimSpace(os.Getenv("TG_BOT_TOKEN")) != "",
		strings.TrimSpace(os.Getenv("DATABASE_URL")) != "",
		strings.TrimSpace(os.Getenv("PLAN_ID")) != "",
		strings.TrimSpace(os.Getenv("WEBHOOK_SECRET")) != "",
		strings.TrimSpace(os.Getenv("ADMIN_TG_ID")) != "",
	)

	token := mustEnv("TG_BOT_TOKEN")
	dbURL := mustEnv("DATABASE_URL")
	planID := mustEnv("PLAN_ID")

	var adminTGID int64
	if v := strings.TrimSpace(os.Getenv("ADMIN_TG_ID")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			log.Printf("ADMIN_TG_ID is invalid, ignoring allow-list: %v", err)
		} else {
			adminTGID = n
		}
	}

	tz := strings.TrimSpace(os.Getenv("TZ"))
	if tz == "" {
		tz = "Europe/Vienna"
	}

	webhookSecret := strings.TrimSpace(os.Getenv("WEBHOOK_SECRET"))
	jobsSecret := strings.TrimSpace(os.Getenv("JOBS_SECRET"))
	jobsTZ := strings.TrimSpace(os.Getenv("JOBS_TZ"))
	if jobsTZ == "" {
		jobsTZ = "Europe/Vienna"
	}
	debugAlwaysShowActions := false
	if v := strings.TrimSpace(os.Getenv("DEBUG_ALWAYS_SHOW_ACTIONS")); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			log.Printf("DEBUG_ALWAYS_SHOW_ACTIONS invalid, using false: %v", err)
		} else {
			debugAlwaysShowActions = b
		}
	}
	debugAllowAnyDay := strings.EqualFold(strings.TrimSpace(os.Getenv("DEBUG_ALLOW_ANY_DAY")), "true")
	log.Printf("LOCAL_DEV raw value: '%s'", os.Getenv("LOCAL_DEV"))
	localDev := strings.EqualFold(strings.TrimSpace(os.Getenv("LOCAL_DEV")), "true")

	ctx := context.Background()

	db, err := NewDB(ctx, dbURL)
	if err != nil {
		log.Fatalf("db init failed: %v", err)
	}
	defer db.Close()

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatalf("telegram init failed: %v", err)
	}
	log.Printf("bot authorized as @%s", bot.Self.UserName)

	a := &app{
		db:                     db,
		bot:                    bot,
		planID:                 planID,
		adminTGID:              adminTGID,
		tz:                     tz,
		debugAlwaysShowActions: debugAlwaysShowActions,
		debugAllowAnyDay:       debugAllowAnyDay,
		webhookSecret:          webhookSecret,
		jobsSecret:             jobsSecret,
		jobsTZ:                 jobsTZ,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/tg/webhook", a.handleTelegramWebhook)
	mux.HandleFunc("/jobs/daily", a.handleJobsDaily)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if localDev {
		log.Println("Running in polling mode")
		go func() {
			log.Printf("Jobs HTTP listening on :%s", port)
			if err := http.ListenAndServe(":"+port, mux); err != nil {
				log.Printf("jobs server stopped: %v", err)
			}
		}()
		a.runPolling(ctx)
		return
	}

	log.Println("Running in webhook mode")
	log.Printf("Listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func (a *app) handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	// Optional shared-secret protection (recommended for public Cloud Run)
	if a.webhookSecret != "" {
		if r.URL.Query().Get("secret") != a.webhookSecret {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var upd tgbotapi.Update
	if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	a.processUpdate(r.Context(), upd)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (a *app) processUpdate(ctx context.Context, upd tgbotapi.Update) {
	if uid, chatID, ok := extractUserChat(&upd); ok {
		if err := a.db.UpsertUser(ctx, uid, chatID); err != nil {
			log.Printf("upsert user failed: %v", err)
		}
	}

	if upd.CallbackQuery != nil {
		a.handleCallbackQuery(ctx, upd.CallbackQuery)
		return
	}

	if upd.Message != nil {
		msg := upd.Message
		if a.adminTGID != 0 && msg.From != nil && msg.From.ID != a.adminTGID {
			return
		}
		if !msg.IsCommand() {
			return
		}

		if err := a.handleCommand(ctx, msg); err != nil {
			log.Printf("command handling failed: %v", err)
		}
	}
}

func (a *app) runPolling(ctx context.Context) {
	_ = ctx
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := a.bot.GetUpdatesChan(u)
	for upd := range updates {
		a.processUpdate(ctx, upd)
	}
}

func (a *app) handleCommand(ctx context.Context, msg *tgbotapi.Message) error {
	cmd := msg.Command()
	args := strings.TrimSpace(msg.CommandArguments())

	switch cmd {
	case "start":
		return a.handleStart(ctx, msg)
	case "plan":
		return a.renderWeekViewMessage(ctx, msg.Chat.ID, 0, 1, false)
	case "help":
		return a.sendText(msg.Chat.ID, helpText())

	case "today":
		return a.sendText(msg.Chat.ID, handleDate(ctx, a.db, a.planID, time.Now(), a.tz))

	case "tomorrow":
		return a.sendText(msg.Chat.ID, handleDate(ctx, a.db, a.planID, time.Now().Add(24*time.Hour), a.tz))

	case "day":
		nStr := strings.TrimSpace(args)
		if nStr == "" {
			return a.sendText(msg.Chat.ID, "❌ Format: /day N\nExample: /day 12")
		}
		n, err := strconv.Atoi(strings.TrimSpace(nStr))
		if err != nil || n <= 0 {
			return a.sendText(msg.Chat.ID, "❌ Format: /day N\nExample: /day 12")
		}

		_, weeks, err := a.db.GetPlanMeta(ctx, a.planID)
		if err != nil {
			return a.sendText(msg.Chat.ID, "❌ DB error: "+err.Error())
		}

		if msg.From == nil {
			return a.sendText(msg.Chat.ID, "User not found.")
		}
		user, ok, err := a.db.GetUser(ctx, msg.From.ID)
		if err != nil {
			return a.sendText(msg.Chat.ID, "❌ DB error: "+err.Error())
		}
		if !ok || user.StartDate == nil {
			return a.sendText(msg.Chat.ID, "Please run /start and pick your start Monday first.")
		}

		dayInfo, err := computeDayInfo(*user.StartDate, strings.TrimSpace(user.TZ), weeks, time.Now())
		if err != nil {
			return a.sendText(msg.Chat.ID, "❌ Failed to compute user day: "+err.Error())
		}

		maxDay := weeks * 7
		if n > maxDay {
			return a.sendText(msg.Chat.ID, "⚠️ Out of range. This plan has "+strconv.Itoa(maxDay)+" days.")
		}

		date := dayInfo.startDate.AddDate(0, 0, n-1)

		day, err := a.db.GetPlanDay(ctx, a.planID, date)
		if err != nil {
			return a.sendText(msg.Chat.ID, "❌ DB error: "+err.Error())
		}
		if day == nil {
			return a.sendText(msg.Chat.ID, "⚠️ No plan entry for Day "+strconv.Itoa(n))
		}

		text := FormatDayMessageWithIndex(n, weeks, date, day)
		return a.sendDayMessage(msg.Chat.ID, text, n, true)

	default:
		return a.sendText(msg.Chat.ID, "Unknown command. Use /help")
	}
}

func (a *app) handleStart(ctx context.Context, msg *tgbotapi.Message) error {
	if msg.From == nil {
		return nil
	}

	user, ok, err := a.db.GetUser(ctx, msg.From.ID)
	if err != nil {
		return a.sendText(msg.Chat.ID, "❌ DB error: "+err.Error())
	}
	if !ok || user.StartDate == nil {
		return a.sendMondayPicker(msg.Chat.ID, "Choose your start Monday")
	}

	text := fmt.Sprintf(
		"Start date: %s",
		user.StartDate.In(viennaLocation()).Format("2006-01-02"),
	)
	msgCfg := tgbotapi.NewMessage(msg.Chat.ID, text)
	msgCfg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Change start date", "start:change"),
		),
	)
	_, err = a.bot.Send(msgCfg)
	return err
}

func (a *app) sendMondayPicker(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = mondayKeyboard()
	_, err := a.bot.Send(msg)
	return err
}

func (a *app) handleCallbackQuery(ctx context.Context, cq *tgbotapi.CallbackQuery) {
	if cq == nil {
		return
	}
	if cq.From != nil && a.adminTGID != 0 && cq.From.ID != a.adminTGID {
		a.answerCallback(cq.ID, "Not allowed")
		return
	}

	if cq.Data == "start:change" {
		if cq.Message == nil {
			a.answerCallback(cq.ID, "No message context")
			return
		}
		edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, "Choose your start Monday")
		kb := mondayKeyboard()
		edit.ReplyMarkup = &kb
		if _, err := a.bot.Send(edit); err != nil {
			log.Printf("failed to edit start-date picker: %v", err)
		}
		a.answerCallback(cq.ID, "OK")
		return
	}

	if strings.HasPrefix(cq.Data, "wk:") {
		if cq.Message == nil {
			a.answerCallback(cq.ID, "No message context")
			return
		}
		weekNumber, err := strconv.Atoi(strings.TrimPrefix(cq.Data, "wk:"))
		if err != nil || weekNumber <= 0 {
			a.answerCallback(cq.ID, "Invalid week")
			return
		}
		if err := a.renderWeekViewMessage(ctx, cq.Message.Chat.ID, cq.Message.MessageID, weekNumber, true); err != nil {
			log.Printf("failed to render week view: %v", err)
			a.answerCallback(cq.ID, "Failed to render week")
			return
		}
		a.answerCallback(cq.ID, "OK")
		return
	}

	if strings.HasPrefix(cq.Data, "day:") {
		if cq.Message == nil {
			a.answerCallback(cq.ID, "No message context")
			return
		}
		if cq.From == nil {
			a.answerCallback(cq.ID, "Missing user")
			return
		}
		dayIndex, err := strconv.Atoi(strings.TrimPrefix(cq.Data, "day:"))
		if err != nil || dayIndex <= 0 {
			a.answerCallback(cq.ID, "Invalid day")
			return
		}
		if err := a.renderDayViewMessage(ctx, cq.Message.Chat.ID, cq.Message.MessageID, cq.From.ID, dayIndex, true); err != nil {
			log.Printf("failed to render day view: %v", err)
			a.answerCallback(cq.ID, "Failed to render day")
			return
		}
		a.answerCallback(cq.ID, "OK")
		return
	}

	if status, dayIndex, ok := parseStatusCallback(cq.Data); ok {
		a.handleStatusCallback(ctx, cq, status, dayIndex)
		return
	}

	if !strings.HasPrefix(cq.Data, "sd:") {
		a.answerCallback(cq.ID, "Unknown action")
		return
	}
	dateStr := strings.TrimPrefix(cq.Data, "sd:")
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		a.answerCallback(cq.ID, "Invalid date format")
		return
	}
	if d.Weekday() != time.Monday {
		a.answerCallback(cq.ID, "Date must be Monday")
		return
	}
	if cq.From == nil {
		a.answerCallback(cq.ID, "Missing user")
		return
	}
	if err := a.db.SetUserStartDate(ctx, cq.From.ID, d); err != nil {
		log.Printf("failed to set start date: %v", err)
		a.answerCallback(cq.ID, "DB error")
		return
	}

	if cq.Message != nil {
		confirm := fmt.Sprintf("Start date set: %s. Try /day 1", d.Format("2006-01-02"))
		edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, confirm)
		if _, err := a.bot.Send(edit); err != nil {
			log.Printf("failed to edit confirmation: %v", err)
		}
	}
	a.answerCallback(cq.ID, "OK")
}

func (a *app) answerCallback(callbackID, text string) {
	cb := tgbotapi.NewCallback(callbackID, text)
	if _, err := a.bot.Request(cb); err != nil {
		log.Printf("failed to answer callback: %v", err)
	}
}

func (a *app) sendDayMessage(chatID int64, text string, dayIndex int, showActions bool) error {
	msg := tgbotapi.NewMessage(chatID, text)
	if showActions {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("✅ Done", fmt.Sprintf("st:done:%d", dayIndex)),
				tgbotapi.NewInlineKeyboardButtonData("❌ Skipped", fmt.Sprintf("st:skipped:%d", dayIndex)),
			),
		)
	}
	_, err := a.bot.Send(msg)
	return err
}

func (a *app) renderWeekView(chatID int64, weekNumber int) {
	if err := a.renderWeekViewMessage(context.Background(), chatID, 0, weekNumber, false); err != nil {
		log.Printf("failed to render week view: %v", err)
	}
}

func (a *app) renderWeekViewMessage(ctx context.Context, chatID int64, messageID int, weekNumber int, edit bool) error {
	startDate, weeks, err := a.db.GetPlanMeta(ctx, a.planID)
	if err != nil {
		return err
	}
	if weeks <= 0 {
		return a.sendText(chatID, "No weeks available.")
	}
	if weekNumber < 1 {
		weekNumber = 1
	}
	if weekNumber > weeks {
		weekNumber = weeks
	}

	maxDay := weeks * 7
	startDayIndex := (weekNumber-1)*7 + 1
	endDayIndex := startDayIndex + 6
	if endDayIndex > maxDay {
		endDayIndex = maxDay
	}

	loc, _ := time.LoadLocation(a.tz)
	if loc == nil {
		loc = time.Local
	}
	baseDate := time.Date(startDate.In(loc).Year(), startDate.In(loc).Month(), startDate.In(loc).Day(), 0, 0, 0, 0, loc)

	dayRow := make([]tgbotapi.InlineKeyboardButton, 0, 7)
	for dayIndex := startDayIndex; dayIndex <= endDayIndex; dayIndex++ {
		dateForDay := baseDate.AddDate(0, 0, dayIndex-1)
		_, _ = a.db.GetPlanDay(ctx, a.planID, dateForDay)
		dayRow = append(dayRow, tgbotapi.NewInlineKeyboardButtonData(strconv.Itoa(dayIndex), fmt.Sprintf("day:%d", dayIndex)))
	}

	kbRows := [][]tgbotapi.InlineKeyboardButton{}
	if len(dayRow) > 0 {
		kbRows = append(kbRows, dayRow)
	}
	navRow := []tgbotapi.InlineKeyboardButton{}
	if weekNumber > 1 {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("◀ Prev Week", fmt.Sprintf("wk:%d", weekNumber-1)))
	}
	if weekNumber < weeks {
		navRow = append(navRow, tgbotapi.NewInlineKeyboardButtonData("Next Week ▶", fmt.Sprintf("wk:%d", weekNumber+1)))
	}
	if len(navRow) > 0 {
		kbRows = append(kbRows, navRow)
	}

	text := fmt.Sprintf("Week %d", weekNumber)
	markup := tgbotapi.NewInlineKeyboardMarkup(kbRows...)

	if edit && messageID != 0 {
		cfg := tgbotapi.NewEditMessageText(chatID, messageID, text)
		cfg.ReplyMarkup = &markup
		_, err := a.bot.Send(cfg)
		return err
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	_, err = a.bot.Send(msg)
	return err
}

func (a *app) renderDayView(chatID int64, dayIndex int) {
	if err := a.renderDayViewMessage(context.Background(), chatID, 0, 0, dayIndex, false); err != nil {
		log.Printf("failed to render day view: %v", err)
	}
}

func (a *app) renderDayViewMessage(ctx context.Context, chatID int64, messageID int, tgUserID int64, dayIndex int, edit bool) error {
	startDate, weeks, err := a.db.GetPlanMeta(ctx, a.planID)
	if err != nil {
		return err
	}
	maxDay := weeks * 7
	if dayIndex < 1 || dayIndex > maxDay {
		return a.sendText(chatID, "Invalid day index.")
	}

	loc, _ := time.LoadLocation(a.tz)
	if loc == nil {
		loc = time.Local
	}
	baseDate := time.Date(startDate.In(loc).Year(), startDate.In(loc).Month(), startDate.In(loc).Day(), 0, 0, 0, 0, loc)
	dateForDay := baseDate.AddDate(0, 0, dayIndex-1)

	day, err := a.db.GetPlanDay(ctx, a.planID, dateForDay)
	if err != nil {
		return err
	}
	if day == nil {
		return a.sendText(chatID, "No plan entry for this day.")
	}

	weekNumber := ((dayIndex - 1) / 7) + 1
	text := fmt.Sprintf("Day %d (Week %d)\n\n%s", dayIndex, weekNumber, FormatDayMessageWithIndex(dayIndex, weeks, dateForDay, day))
	completed := false
	if tgUserID != 0 {
		completion, ok, err := a.db.GetCompletion(ctx, tgUserID, a.planID, dayIndex)
		if err != nil {
			return err
		}
		if ok {
			completed = true
			text += "\n\n" + formatCompletionStatus(completion.Status)
		}
	}

	rows := [][]tgbotapi.InlineKeyboardButton{}
	if completed {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅ Back to Week", fmt.Sprintf("wk:%d", weekNumber)),
		))
	} else {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅ Back to Week", fmt.Sprintf("wk:%d", weekNumber)),
			tgbotapi.NewInlineKeyboardButtonData("✅ Done", fmt.Sprintf("st:done:%d", dayIndex)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Skipped", fmt.Sprintf("st:skipped:%d", dayIndex)),
		))
	}
	markup := tgbotapi.NewInlineKeyboardMarkup(rows...)

	if edit && messageID != 0 {
		cfg := tgbotapi.NewEditMessageText(chatID, messageID, text)
		cfg.ReplyMarkup = &markup
		_, err := a.bot.Send(cfg)
		return err
	}
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = markup
	_, err = a.bot.Send(msg)
	return err
}

func formatCompletionStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "DONE":
		return "Status: DONE ✅"
	case "SKIPPED":
		return "Status: SKIPPED ❌"
	default:
		return "Status: " + strings.ToUpper(strings.TrimSpace(status))
	}
}

func (a *app) sendText(chatID int64, text string) error {
	out := tgbotapi.NewMessage(chatID, text)
	_, err := a.bot.Send(out)
	return err
}

func (a *app) handleStatusCallback(ctx context.Context, cq *tgbotapi.CallbackQuery, status string, dayIndex int) {
	if cq.From == nil {
		a.answerCallback(cq.ID, "Missing user")
		return
	}
	if cq.Message == nil {
		a.answerCallback(cq.ID, "No message context")
		return
	}

	user, ok, err := a.db.GetUser(ctx, cq.From.ID)
	if err != nil {
		log.Printf("failed to fetch user: %v", err)
		a.answerCallback(cq.ID, "DB error")
		return
	}
	if !ok || user.StartDate == nil {
		a.answerCallback(cq.ID, "Please run /start first.")
		return
	}

	_, weeks, err := a.db.GetPlanMeta(ctx, a.planID)
	if err != nil {
		log.Printf("failed to fetch plan meta: %v", err)
		a.answerCallback(cq.ID, "DB error")
		return
	}

	dayInfo, err := computeDayInfo(*user.StartDate, strings.TrimSpace(user.TZ), weeks, time.Now())
	if err != nil {
		log.Printf("failed to compute day info: %v", err)
		a.answerCallback(cq.ID, "Time error")
		return
	}

	if !a.debugAllowAnyDay {
		if dayIndex > dayInfo.todayDayIndex {
			a.answerCallback(cq.ID, "Too early for this day.")
			return
		}
	}

	if err := a.db.InsertCompletion(ctx, cq.From.ID, a.planID, dayIndex, status); err != nil {
		log.Printf("failed to insert completion: %v", err)
		a.answerCallback(cq.ID, "DB error")
		return
	}

	updatedText := strings.TrimRight(cq.Message.Text, "\n") + "\nStatus: " + strings.ToUpper(status)
	edit := tgbotapi.NewEditMessageText(cq.Message.Chat.ID, cq.Message.MessageID, updatedText)
	empty := tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}}
	edit.ReplyMarkup = &empty
	if _, err := a.bot.Send(edit); err != nil {
		log.Printf("failed to edit message after status: %v", err)
	}
	a.answerCallback(cq.ID, "OK")
}

func extractUserChat(upd *tgbotapi.Update) (int64, int64, bool) {
	if upd == nil {
		return 0, 0, false
	}
	if upd.Message != nil && upd.Message.From != nil {
		return upd.Message.From.ID, upd.Message.Chat.ID, true
	}
	if upd.CallbackQuery != nil && upd.CallbackQuery.From != nil && upd.CallbackQuery.Message != nil {
		return upd.CallbackQuery.From.ID, upd.CallbackQuery.Message.Chat.ID, true
	}
	return 0, 0, false
}

func mondayKeyboard() tgbotapi.InlineKeyboardMarkup {
	next := nextMondays(4)
	rows := make([][]tgbotapi.InlineKeyboardButton, 0, len(next))
	for _, d := range next {
		dateStr := d.Format("2006-01-02")
		btn := tgbotapi.NewInlineKeyboardButtonData("Mon "+dateStr, "sd:"+dateStr)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...)
}

type dayInfo struct {
	startDate     time.Time
	todayDayIndex int
}

func computeDayInfo(userStartDate time.Time, userTZ string, weeks int, now time.Time) (dayInfo, error) {
	loc, err := resolveUserLocation(userTZ)
	if err != nil {
		return dayInfo{}, err
	}
	startLocal := time.Date(
		userStartDate.In(loc).Year(),
		userStartDate.In(loc).Month(),
		userStartDate.In(loc).Day(),
		0, 0, 0, 0,
		loc,
	)
	todayLocal := time.Date(
		now.In(loc).Year(),
		now.In(loc).Month(),
		now.In(loc).Day(),
		0, 0, 0, 0,
		loc,
	)

	diffDays := int(todayLocal.Sub(startLocal).Hours() / 24)
	maxDay := weeks * 7
	todayIdx := diffDays + 1
	if todayIdx < 1 {
		todayIdx = 0
	}
	if todayIdx > maxDay {
		todayIdx = maxDay + 1
	}
	return dayInfo{
		startDate:     startLocal,
		todayDayIndex: todayIdx,
	}, nil
}

func parseStatusCallback(data string) (string, int, bool) {
	parts := strings.Split(data, ":")
	if len(parts) != 3 || parts[0] != "st" {
		return "", 0, false
	}
	status := parts[1]
	if status != "done" && status != "skipped" {
		return "", 0, false
	}
	dayIndex, err := strconv.Atoi(parts[2])
	if err != nil || dayIndex <= 0 {
		return "", 0, false
	}
	return status, dayIndex, true
}

func nextMondays(n int) []time.Time {
	out := make([]time.Time, 0, n)
	now := time.Now().In(viennaLocation())
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	offset := (int(time.Monday) - int(day.Weekday()) + 7) % 7
	first := day.AddDate(0, 0, offset)
	for i := 0; i < n; i++ {
		out = append(out, first.AddDate(0, 0, i*7))
	}
	return out
}

func viennaLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Vienna")
	if err != nil {
		return time.FixedZone("Europe/Vienna", 1*60*60)
	}
	return loc
}

func resolveUserLocation(tz string) (*time.Location, error) {
	if strings.TrimSpace(tz) == "" {
		return viennaLocation(), nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, err
	}
	return loc, nil
}

func handleDate(ctx context.Context, db *DB, planID string, t time.Time, tz string) string {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}
	date := time.Date(t.In(loc).Year(), t.In(loc).Month(), t.In(loc).Day(), 0, 0, 0, 0, loc)

	day, err := db.GetPlanDay(ctx, planID, date)
	if err != nil {
		return "❌ DB error: " + err.Error()
	}
	if day == nil {
		return "⚠️ No plan entry for " + date.Format("2006-01-02")
	}

	return FormatDayMessage(date, day)
}

func helpText() string {
	return strings.TrimSpace(`
		Fitness Bot commands:
		- /start — set or change your start Monday
		- /plan — open week/day menu
		- /day N — plan by day index (Example: /day 12)
		- /today — today plan
		- /tomorrow — tomorrow plan
		- /help
		`)
}

func mustEnv(k string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		log.Fatalf("missing env: %s", k)
	}
	return v
}

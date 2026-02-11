package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	// load .env (ignore error in prod)
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found, using system env")
	}
	token := mustEnv("TG_BOT_TOKEN")
	dbURL := mustEnv("DATABASE_URL")
	planID := mustEnv("PLAN_ID")

	var adminTGID int64
	if v := strings.TrimSpace(os.Getenv("ADMIN_TG_ID")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			log.Fatalf("ADMIN_TG_ID invalid: %v", err)
		}
		adminTGID = n
	}

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

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for upd := range updates {
		if upd.Message == nil {
			continue
		}
		msg := upd.Message

		if adminTGID != 0 && msg.From != nil && msg.From.ID != adminTGID {
			// Ignore чужих
			continue
		}

		if !msg.IsCommand() {
			continue
		}

		cmd := msg.Command()
		args := strings.TrimSpace(msg.CommandArguments())

		var reply string

		switch cmd {
		case "start", "help":
			reply = helpText()

		case "today":
			reply = handleDate(ctx, db, planID, time.Now(), "Europe/Vienna")

		case "tomorrow":
			reply = handleDate(ctx, db, planID, time.Now().Add(24*time.Hour), "Europe/Vienna")

		case "date":
			// /date 2026-02-16
			d, err := time.Parse("2006-01-02", strings.TrimSpace(args))
			if err != nil {
				reply = "❌ Format: /date YYYY-MM-DD\nExample: /date 2026-02-16"
				break
			}
			reply = handleDate(ctx, db, planID, d, "Europe/Vienna")
		case "day":
			// /day 12
			nStr := strings.TrimSpace(msg.CommandArguments())
			if nStr == "" {
				parts := strings.Fields(msg.Text)
				if len(parts) >= 2 {
					nStr = parts[1]
				}
			}
			n, err := strconv.Atoi(strings.TrimSpace(nStr))
			if err != nil || n <= 0 {
				reply = "❌ Format: /day N\nExample: /day 12"
				break
			}

			startDate, weeks, err := db.GetPlanMeta(ctx, planID)
			if err != nil {
				reply = "❌ DB error: " + err.Error()
				break
			}

			maxDay := weeks * 7
			if n > maxDay {
				reply = "⚠️ Out of range. This plan has " + strconv.Itoa(maxDay) + " days."
				break
			}

			loc, _ := time.LoadLocation("Europe/Vienna")
			date := time.Date(startDate.In(loc).Year(), startDate.In(loc).Month(), startDate.In(loc).Day(), 0, 0, 0, 0, loc).
				AddDate(0, 0, n-1)

			day, err := db.GetPlanDay(ctx, planID, date)
			if err != nil {
				reply = "❌ DB error: " + err.Error()
				break
			}
			if day == nil {
				reply = "⚠️ No plan entry for Day " + strconv.Itoa(n)
				break
			}

			reply = FormatDayMessageWithIndex(n, weeks, date, day)

		default:
			reply = "Unknown command. Use /help"
		}

		out := tgbotapi.NewMessage(msg.Chat.ID, reply)
		out.ParseMode = "Markdown"
		_, _ = bot.Send(out)
	}
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
*Fitness Bot* commands:
- /today — today plan
- /tomorrow — tomorrow plan
- /date YYYY-MM-DD — plan for date
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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type jobsDailyResponse struct {
	OK           bool                  `json:"ok"`
	AlreadyRan   bool                  `json:"already_ran,omitempty"`
	RunID        string                `json:"run_id,omitempty"`
	TZ           string                `json:"tz,omitempty"`
	NowLocal     string                `json:"now_local,omitempty"`
	DryRun       bool                  `json:"dry_run,omitempty"`
	Processed    int                   `json:"processed_users,omitempty"`
	Sent         int                   `json:"sent_messages,omitempty"`
	Error        string                `json:"error,omitempty"`
	Deliveries   []jobsDeliveryPreview `json:"deliveries,omitempty"`
}

type jobsDeliveryPreview struct {
	TGUserID int64  `json:"tg_user_id"`
	TGChatID int64  `json:"tg_chat_id"`
	DayIndex int    `json:"day_index"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

// curl -X POST "http://localhost:8080/jobs/daily?secret=...&dry=1"
// curl -X POST "https://<cloudrun>/jobs/daily?secret=..."
func (a *app) handleJobsDaily(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.jobsSecret == "" || r.URL.Query().Get("secret") != a.jobsSecret {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	dryRun := isDryRun(r.URL.Query().Get("dry"))
	force := strings.EqualFold(strings.TrimSpace(os.Getenv("CRON_FORCE_SEND")), "true")
	tz := strings.TrimSpace(a.jobsTZ)
	if tz == "" {
		tz = "Europe/Vienna"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		writeJobsJSON(w, http.StatusInternalServerError, jobsDailyResponse{
			OK:    false,
			Error: "invalid jobs timezone",
		})
		return
	}
	now := time.Now().In(loc)
	nowLocal := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	ctx := r.Context()
	runID, created, err := a.db.CreateCronRun(ctx, "daily", nowLocal, tz, dryRun)
	if err != nil {
		writeJobsJSON(w, http.StatusInternalServerError, jobsDailyResponse{
			OK:       false,
			Error:    "failed to create cron run",
			TZ:       tz,
			NowLocal: nowLocal.Format("2006-01-02"),
			DryRun:   dryRun,
		})
		return
	}
	if !created {
		writeJobsJSON(w, http.StatusOK, jobsDailyResponse{
			OK:         true,
			AlreadyRan: true,
			RunID:      runID,
			TZ:         tz,
			NowLocal:   nowLocal.Format("2006-01-02"),
			DryRun:     dryRun,
		})
		return
	}

	processed := 0
	sent := 0
	deliveries := make([]jobsDeliveryPreview, 0, 20)
	runStatus := "completed"
	var runErr *string
	defer func() {
		_ = a.db.FinishCronRun(context.Background(), runID, runStatus, processed, sent, runErr)
	}()

	users, err := a.db.ListUsers(ctx)
	if err != nil {
		e := "failed to list users"
		runStatus = "failed"
		runErr = &e
		writeJobsJSON(w, http.StatusInternalServerError, jobsDailyResponse{
			OK:       false,
			RunID:    runID,
			Error:    e,
			TZ:       tz,
			NowLocal: nowLocal.Format("2006-01-02"),
			DryRun:   dryRun,
		})
		return
	}

	weeks, err := a.db.GetPlanWeeks(ctx, a.planID)
	if err != nil {
		e := "failed to load plan weeks"
		runStatus = "failed"
		runErr = &e
		writeJobsJSON(w, http.StatusInternalServerError, jobsDailyResponse{
			OK:       false,
			RunID:    runID,
			Error:    e,
			TZ:       tz,
			NowLocal: nowLocal.Format("2006-01-02"),
			DryRun:   dryRun,
		})
		return
	}

	for _, u := range users {
		processed++
		dayIndex := 0
		decision := "skipped"
		reason := ""

		logDelivery := func(msgID *int64, errText *string) {
			_ = a.db.InsertCronDelivery(
				ctx,
				runID,
				u.TGUserID,
				u.TGChatID,
				a.planID,
				dayIndex,
				decision,
				reason,
				msgID,
				errText,
			)
			if len(deliveries) < 20 {
				deliveries = append(deliveries, jobsDeliveryPreview{
					TGUserID: u.TGUserID,
					TGChatID: u.TGChatID,
					DayIndex: dayIndex,
					Decision: decision,
					Reason:   reason,
				})
			}
		}

		if force {
			dayIndex = 1

			if dryRun {
				decision = "skipped"
				reason = "dry_run"
				logDelivery(nil, nil)
				continue
			}

			planStartDate, _, err := a.db.GetPlanMeta(ctx, a.planID)
			if err != nil {
				decision = "forced_meta_error"
				reason = "forced_meta_error"
				errText := err.Error()
				logDelivery(nil, &errText)
				continue
			}

			planDay, err := a.db.GetPlanDay(ctx, a.planID, planStartDate)
			if err != nil {
				decision = "forced_no_plan_day"
				reason = "forced_no_plan_day"
				errText := err.Error()
				logDelivery(nil, &errText)
				continue
			}
			if planDay == nil {
				decision = "forced_no_plan_day"
				reason = "forced_no_plan_day"
				logDelivery(nil, nil)
				continue
			}

			text := formatCronMessage(dayIndex, weeks, planStartDate, planDay)
			if err := a.sendDayMessage(u.TGChatID, text, dayIndex, true); err != nil {
				decision = "forced_send_error"
				reason = "forced_send_error"
				errText := err.Error()
				logDelivery(nil, &errText)
				continue
			}

			if err := a.db.InsertNotification(ctx, u.TGUserID, a.planID, dayIndex, nowLocal); err != nil {
				decision = "forced_notification_insert_failed"
				reason = "forced_notification_insert_failed"
				errText := err.Error()
				logDelivery(nil, &errText)
				continue
			}

			decision = "forced_send"
			reason = "forced_send"
			logDelivery(nil, nil)
			sent++
			continue
		}

		if u.StartDate == nil {
			reason = "no_start_date"
			logDelivery(nil, nil)
			continue
		}

		userTZ := strings.TrimSpace(u.TZ)
		if userTZ == "" {
			userTZ = tz
		}
		info, err := computeDayInfo(*u.StartDate, userTZ, weeks, time.Now())
		if err != nil {
			reason = "invalid_user_tz"
			errText := err.Error()
			logDelivery(nil, &errText)
			continue
		}
		dayIndex = info.todayDayIndex
		maxDay := weeks * 7
		if dayIndex < 1 || dayIndex > maxDay {
			reason = "out_of_range"
			logDelivery(nil, nil)
			continue
		}

		hasNotif, err := a.db.HasNotification(ctx, u.TGUserID, a.planID, dayIndex)
		if err != nil {
			reason = "notification_check_failed"
			errText := err.Error()
			logDelivery(nil, &errText)
			continue
		}
		if hasNotif {
			reason = "already_sent"
			logDelivery(nil, nil)
			continue
		}

		hasCompletion, err := a.db.HasCompletion(ctx, u.TGUserID, a.planID, dayIndex)
		if err != nil {
			reason = "completion_check_failed"
			errText := err.Error()
			logDelivery(nil, &errText)
			continue
		}
		if hasCompletion {
			reason = "already_completed"
			logDelivery(nil, nil)
			continue
		}

		if dryRun {
			reason = "dry_run"
			logDelivery(nil, nil)
			continue
		}

		dateForDay := info.startDate.AddDate(0, 0, dayIndex-1)
		planDay, err := a.db.GetPlanDayByDate(ctx, a.planID, dateForDay)
		if err != nil {
			reason = "plan_day_load_failed"
			errText := err.Error()
			logDelivery(nil, &errText)
			continue
		}
		if planDay == nil {
			reason = "no_plan_day"
			logDelivery(nil, nil)
			continue
		}

		text := formatCronMessage(dayIndex, weeks, dateForDay, planDay)
		if err := a.sendDayMessage(u.TGChatID, text, dayIndex, true); err != nil {
			reason = "telegram_send_failed"
			errText := err.Error()
			logDelivery(nil, &errText)
			continue
		}

		if err := a.db.InsertNotification(ctx, u.TGUserID, a.planID, dayIndex, nowLocal); err != nil {
			reason = "notification_insert_failed"
			errText := err.Error()
			logDelivery(nil, &errText)
			continue
		}

		decision = "sent"
		reason = "sent"
		logDelivery(nil, nil)
		sent++
	}

	writeJobsJSON(w, http.StatusOK, jobsDailyResponse{
		OK:         true,
		RunID:      runID,
		TZ:         tz,
		NowLocal:   nowLocal.Format("2006-01-02"),
		DryRun:     dryRun,
		Processed:  processed,
		Sent:       sent,
		Deliveries: deliveries,
	})
}

func isDryRun(raw string) bool {
	raw = strings.TrimSpace(strings.ToLower(raw))
	return raw == "1" || raw == "true" || raw == "yes"
}

func writeJobsJSON(w http.ResponseWriter, status int, payload jobsDailyResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func toDayLabel(dayIndex int) string {
	return strconv.Itoa(dayIndex)
}

func formatCronMessage(dayIndex, weeks int, dateForDay time.Time, day *PlanDay) string {
	if day == nil {
		return fmt.Sprintf("Day %s", toDayLabel(dayIndex))
	}
	return FormatDayMessageWithIndex(dayIndex, weeks, dateForDay, day)
}

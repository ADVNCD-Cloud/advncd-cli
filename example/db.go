package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

type User struct {
	TGUserID  int64
	TGChatID  int64
	TZ        string
	StartDate *time.Time
}

type Completion struct {
	TGUserID  int64
	ProgramID string
	DayIndex  int
	Status    string
}

func (d *DB) GetPlanMeta(ctx context.Context, planID string) (time.Time, int, error) {
	const q = `select start_date, weeks from plans where id = $1 limit 1`
	var startDate time.Time
	var weeks int
	err := d.pool.QueryRow(ctx, q, planID).Scan(&startDate, &weeks)
	if err != nil {
		return time.Time{}, 0, err
	}
	return startDate, weeks, nil
}

func NewDB(ctx context.Context, url string) (*DB, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	// ping
	c, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(c); err != nil {
		pool.Close()
		return nil, err
	}
	db := &DB{pool: pool}
	if err := db.ensureSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return db, nil
}

func (d *DB) Close() { d.pool.Close() }

type PlanDay struct {
	DayType string         `json:"day_type"`
	Workout map[string]any `json:"workout"`
	Meals   map[string]any `json:"meals"`
}

func (d *DB) GetPlanDay(ctx context.Context, planID string, date time.Time) (*PlanDay, error) {
	const q = `
select day_type, workout, meals
from plan_days
where plan_id = $1 and date = $2
limit 1
`
	row := d.pool.QueryRow(ctx, q, planID, date.Format("2006-01-02"))

	var dayType string
	var workoutJSON []byte
	var mealsJSON []byte

	if err := row.Scan(&dayType, &workoutJSON, &mealsJSON); err != nil {
		// not found => nil, nil
		// pgx returns pgx.ErrNoRows
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}

	out := &PlanDay{DayType: dayType}

	if len(workoutJSON) > 0 {
		_ = json.Unmarshal(workoutJSON, &out.Workout)
	}
	if len(mealsJSON) > 0 {
		_ = json.Unmarshal(mealsJSON, &out.Meals)
	}
	return out, nil
}

func (d *DB) ensureSchema(ctx context.Context) error {
	const usersSQL = `
create table if not exists users (
	tg_user_id bigint primary key,
	tg_chat_id bigint not null,
	tz text not null default 'Europe/Vienna',
	start_date date null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now()
)`
	if _, err := d.pool.Exec(ctx, usersSQL); err != nil {
		return err
	}

	const completionsSQL = `
create table if not exists day_completions (
	tg_user_id bigint not null references users(tg_user_id) on delete cascade,
	program_id text not null,
	day_index int not null,
	status text not null check (status in ('done','skipped')),
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	unique (tg_user_id, program_id, day_index)
)`
	_, err := d.pool.Exec(ctx, completionsSQL)
	return err
}

func (d *DB) UpsertUser(ctx context.Context, tgUserID, tgChatID int64) error {
	const q = `
insert into users (tg_user_id, tg_chat_id)
values ($1, $2)
on conflict (tg_user_id)
do update set tg_chat_id = excluded.tg_chat_id, updated_at = now()
`
	_, err := d.pool.Exec(ctx, q, tgUserID, tgChatID)
	return err
}

func (d *DB) GetUser(ctx context.Context, tgUserID int64) (User, bool, error) {
	const q = `
select tg_user_id, tg_chat_id, tz, start_date
from users
where tg_user_id = $1
limit 1
`
	var u User
	var startDate *time.Time
	err := d.pool.QueryRow(ctx, q, tgUserID).Scan(&u.TGUserID, &u.TGChatID, &u.TZ, &startDate)
	if err != nil {
		if err == pgx.ErrNoRows {
			return User{}, false, nil
		}
		return User{}, false, err
	}
	u.StartDate = startDate
	return u, true, nil
}

func (d *DB) GetUserStartDate(ctx context.Context, tgUserID int64) (*time.Time, string, bool, error) {
	u, ok, err := d.GetUser(ctx, tgUserID)
	if err != nil {
		return nil, "", false, err
	}
	if !ok {
		return nil, "", false, nil
	}
	return u.StartDate, u.TZ, true, nil
}

func (d *DB) SetUserStartDate(ctx context.Context, tgUserID int64, startDate time.Time) error {
	const q = `
update users
set start_date = $2::date, updated_at = now()
where tg_user_id = $1
`
	_, err := d.pool.Exec(ctx, q, tgUserID, startDate.Format("2006-01-02"))
	return err
}

func (d *DB) GetCompletion(ctx context.Context, tgUserID int64, programID string, dayIndex int) (Completion, bool, error) {
	const q = `
select tg_user_id, program_id, day_index, status
from day_completions
where tg_user_id = $1 and program_id = $2 and day_index = $3
limit 1
`
	var c Completion
	err := d.pool.QueryRow(ctx, q, tgUserID, programID, dayIndex).Scan(&c.TGUserID, &c.ProgramID, &c.DayIndex, &c.Status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return Completion{}, false, nil
		}
		return Completion{}, false, err
	}
	return c, true, nil
}

func (d *DB) InsertCompletion(ctx context.Context, tgUserID int64, programID string, dayIndex int, status string) error {
	const q = `
insert into day_completions (tg_user_id, program_id, day_index, status)
values ($1, $2, $3, $4)
on conflict (tg_user_id, program_id, day_index) do nothing
`
	_, err := d.pool.Exec(ctx, q, tgUserID, programID, dayIndex, status)
	return err
}

func (d *DB) ListUsers(ctx context.Context) ([]User, error) {
	const q = `
select tg_user_id, tg_chat_id, tz, start_date
from users
where tg_chat_id is not null and tg_chat_id <> 0
order by tg_user_id
`
	rows, err := d.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]User, 0)
	for rows.Next() {
		var u User
		var startDate *time.Time
		if err := rows.Scan(&u.TGUserID, &u.TGChatID, &u.TZ, &startDate); err != nil {
			return nil, err
		}
		u.StartDate = startDate
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) GetPlanWeeks(ctx context.Context, planID string) (int, error) {
	const q = `select weeks from plans where id = $1::uuid limit 1`
	var weeks int
	if err := d.pool.QueryRow(ctx, q, planID).Scan(&weeks); err != nil {
		return 0, err
	}
	return weeks, nil
}

func (d *DB) HasNotification(ctx context.Context, tgUserID int64, planID string, dayIndex int) (bool, error) {
	const q = `
select 1
from day_notifications
where tg_user_id = $1 and plan_id = $2::uuid and day_index = $3
limit 1
`
	var one int
	err := d.pool.QueryRow(ctx, q, tgUserID, planID, dayIndex).Scan(&one)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *DB) InsertNotification(ctx context.Context, tgUserID int64, planID string, dayIndex int, nowLocal time.Time) error {
	const q = `
insert into day_notifications (tg_user_id, plan_id, day_index, now_local)
values ($1, $2::uuid, $3, $4::date)
on conflict (tg_user_id, plan_id, day_index) do nothing
`
	_, err := d.pool.Exec(ctx, q, tgUserID, planID, dayIndex, nowLocal.Format("2006-01-02"))
	return err
}

func (d *DB) HasCompletion(ctx context.Context, tgUserID int64, planID string, dayIndex int) (bool, error) {
	const q = `
select 1
from day_completions
where tg_user_id = $1 and plan_id = $2::uuid and day_index = $3
limit 1
`
	var one int
	err := d.pool.QueryRow(ctx, q, tgUserID, planID, dayIndex).Scan(&one)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *DB) CreateCronRun(ctx context.Context, jobName string, nowLocal time.Time, tz string, dry bool) (string, bool, error) {
	const ins = `
insert into cron_runs (job_name, now_local, tz, dry_run, status, created_at)
values ($1, $2::date, $3, $4, 'running', now())
on conflict (job_name, now_local, tz, dry_run) do nothing
returning id
`
	var runID string
	err := d.pool.QueryRow(ctx, ins, jobName, nowLocal.Format("2006-01-02"), tz, dry).Scan(&runID)
	if err == nil {
		return runID, true, nil
	}
	if err != pgx.ErrNoRows {
		return "", false, err
	}

	const sel = `
select id
from cron_runs
where job_name = $1 and now_local = $2::date and tz = $3 and dry_run = $4
limit 1
`
	if err := d.pool.QueryRow(ctx, sel, jobName, nowLocal.Format("2006-01-02"), tz, dry).Scan(&runID); err != nil {
		return "", false, err
	}
	return runID, false, nil
}

func (d *DB) InsertCronDelivery(
	ctx context.Context,
	runID string,
	tgUserID int64,
	tgChatID int64,
	planID string,
	dayIndex int,
	decision, reason string,
	msgID *int64,
	errText *string,
) error {
	const q = `
insert into cron_deliveries (
	cron_run_id, tg_user_id, tg_chat_id, plan_id, day_index,
	decision, reason, telegram_message_id, error
) values (
	$1::uuid, $2, $3, $4::uuid, $5, $6, $7, $8, $9
)
`
	_, err := d.pool.Exec(ctx, q, runID, tgUserID, tgChatID, planID, dayIndex, decision, reason, msgID, errText)
	return err
}

func (d *DB) FinishCronRun(ctx context.Context, runID, status string, processed int, sent int, errText *string) error {
	const q = `
update cron_runs
set status = $2,
    error = $3,
    processed_users = $4,
    sent_messages = $5,
    finished_at = now()
where id = $1::uuid
`
	_, err := d.pool.Exec(ctx, q, runID, status, errText, processed, sent)
	return err
}

func (d *DB) GetPlanDayByDate(ctx context.Context, planID string, date time.Time) (*PlanDay, error) {
	const q = `
select day_type, workout, meals
from plan_days
where plan_id = $1::uuid and date = $2::date
limit 1
`
	row := d.pool.QueryRow(ctx, q, planID, date.Format("2006-01-02"))

	var dayType string
	var workoutJSON []byte
	var mealsJSON []byte
	if err := row.Scan(&dayType, &workoutJSON, &mealsJSON); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	out := &PlanDay{DayType: dayType}
	if len(workoutJSON) > 0 {
		_ = json.Unmarshal(workoutJSON, &out.Workout)
	}
	if len(mealsJSON) > 0 {
		_ = json.Unmarshal(mealsJSON, &out.Meals)
	}
	return out, nil
}

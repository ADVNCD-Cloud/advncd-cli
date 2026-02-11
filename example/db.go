package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
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
	return &DB{pool: pool}, nil
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

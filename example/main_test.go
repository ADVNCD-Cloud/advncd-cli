package main

import (
	"testing"
	"time"
)

func TestComputeDayInfo_BasicAndBounds(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 2, 10, 15, 0, 0, 0, time.UTC)
	weeks := 2 // max day = 14

	info, err := computeDayInfo(start, "UTC", weeks, time.Date(2026, 2, 10, 23, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("computeDayInfo error: %v", err)
	}
	if info.todayDayIndex != 1 {
		t.Fatalf("todayDayIndex mismatch: got %d want %d", info.todayDayIndex, 1)
	}
	if !info.startDate.Equal(time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("startDate must be normalized to midnight, got %v", info.startDate)
	}

	before, err := computeDayInfo(start, "UTC", weeks, time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("computeDayInfo(before) error: %v", err)
	}
	if before.todayDayIndex != 0 {
		t.Fatalf("before-start day index mismatch: got %d want %d", before.todayDayIndex, 0)
	}

	after, err := computeDayInfo(start, "UTC", weeks, time.Date(2026, 3, 20, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("computeDayInfo(after) error: %v", err)
	}
	if after.todayDayIndex != weeks*7+1 {
		t.Fatalf("after-plan day index mismatch: got %d want %d", after.todayDayIndex, weeks*7+1)
	}
}

func TestComputeDayInfo_InvalidTimezone(t *testing.T) {
	t.Parallel()

	_, err := computeDayInfo(time.Now(), "Bad/Timezone", 1, time.Now())
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestParseStatusCallback(t *testing.T) {
	t.Parallel()

	status, day, ok := parseStatusCallback("st:done:12")
	if !ok || status != "done" || day != 12 {
		t.Fatalf("valid callback parse failed: ok=%v status=%q day=%d", ok, status, day)
	}

	_, _, ok = parseStatusCallback("st:later:12")
	if ok {
		t.Fatal("expected invalid status to be rejected")
	}

	_, _, ok = parseStatusCallback("st:done:0")
	if ok {
		t.Fatal("expected non-positive day to be rejected")
	}
}

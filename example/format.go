package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func FormatDayMessageWithIndex(dayN int, weeks int, date time.Time, day *PlanDay) string {
	weekN := ((dayN - 1) / 7) + 1

	var b strings.Builder

	fmt.Fprintf(&b, "*Day %d / Week %d* — %s\n",
		dayN,
		weekN,
		date.Format("Mon 2006-01-02"),
	)

	title := strings.ToUpper(day.DayType)
	if wt := workoutTitle(day.Workout); wt != "" {
		title = wt
	}
	fmt.Fprintf(&b, "%s\n", escapeMD(title))

	// Meals
	if len(day.Meals) > 0 {
		b.WriteString("\n*🍽 Meals*\n")
		for _, k := range orderedMealKeys(day.Meals) {
			v := fmt.Sprintf("%v", day.Meals[k])
			if v == "" || v == "<nil>" {
				continue
			}
			fmt.Fprintf(&b, "- *%s:* %s\n",
				humanMealKey(k),
				escapeMD(v),
			)
		}
	}

	// Workout
	items := workoutItems(day.Workout)
	if len(items) > 0 {
		b.WriteString("\n*🏋️ Workout*\n")
		for _, it := range items {
			fmt.Fprintf(&b, "- %s\n", escapeMD(it))
		}
	}

	return b.String()
}

func FormatDayMessage(date time.Time, day *PlanDay) string {
	var b strings.Builder

	title := strings.ToUpper(day.DayType)
	if wt := workoutTitle(day.Workout); wt != "" {
		title = wt
	}

	fmt.Fprintf(&b, "*%s* — %s\n", date.Format("Mon 2006-01-02"), title)

	// Meals
	if len(day.Meals) > 0 {
		b.WriteString("\n*🍽 Meals*\n")
		for _, k := range orderedMealKeys(day.Meals) {
			v := fmt.Sprintf("%v", day.Meals[k])
			if v == "" || v == "<nil>" {
				continue
			}
			fmt.Fprintf(&b, "- *%s:* %s\n", humanMealKey(k), escapeMD(v))
		}
	}

	// Workout
	items := workoutItems(day.Workout)
	if len(items) > 0 || workoutTitle(day.Workout) != "" {
		b.WriteString("\n*🏋️ Workout*\n")
		if t := workoutTitle(day.Workout); t != "" {
			fmt.Fprintf(&b, "_%s_\n", escapeMD(t))
		}
		for _, it := range items {
			fmt.Fprintf(&b, "- %s\n", escapeMD(it))
		}
	}

	return b.String()
}

func workoutTitle(w map[string]any) string {
	if w == nil {
		return ""
	}
	if v, ok := w["title"].(string); ok {
		return v
	}
	return ""
}

func workoutItems(w map[string]any) []string {
	if w == nil {
		return nil
	}
	raw, ok := w["items"]
	if !ok {
		return nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		out = append(out, fmt.Sprintf("%v", x))
	}
	return out
}

func orderedMealKeys(m map[string]any) []string {
	// preferred order
	order := []string{"breakfast", "snack", "lunch", "pre_workout", "post_workout", "dinner", "note"}
	seen := map[string]bool{}
	out := make([]string, 0, len(m))

	for _, k := range order {
		if _, ok := m[k]; ok {
			out = append(out, k)
			seen[k] = true
		}
	}
	// rest keys
	var rest []string
	for k := range m {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	out = append(out, rest...)
	return out
}

func humanMealKey(k string) string {
	switch k {
	case "breakfast":
		return "Breakfast"
	case "snack":
		return "Snack"
	case "lunch":
		return "Lunch"
	case "pre_workout":
		return "Pre-workout"
	case "post_workout":
		return "Post-workout"
	case "dinner":
		return "Dinner"
	case "note":
		return "Note"
	default:
		return k
	}
}

// super minimal markdown escaping for Telegram (Markdown mode)
func escapeMD(s string) string {
	repl := []string{"_", "\\_", "*", "\\*", "`", "\\`", "[", "\\[", "]", "\\]"}
	r := strings.NewReplacer(repl...)
	return r.Replace(s)
}

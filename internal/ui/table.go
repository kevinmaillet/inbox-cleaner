package ui

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/kevinmaillet/inbox-cleaner/internal/subscriptions"
)

func PrintSubscriptions(w io.Writer, items []subscriptions.Subscription, now time.Time) error {
	table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "COUNT\tLAST RECEIVED\tSUBSCRIPTION"); err != nil {
		return err
	}
	for _, item := range items {
		if _, err := fmt.Fprintf(table, "%d\t%s\t%s\n", item.MessageCount, relativeDate(item.LastReceived, now), item.Name); err != nil {
			return err
		}
	}
	return table.Flush()
}

func relativeDate(date, now time.Time) string {
	if date.IsZero() {
		return "unknown"
	}
	localDate := date.In(now.Location())
	if localDate.Year() == now.Year() && localDate.YearDay() == now.YearDay() {
		return "today"
	}
	if localDate.Year() == now.Year() {
		return localDate.Format("Jan 2")
	}
	return localDate.Format("Jan 2, 2006")
}

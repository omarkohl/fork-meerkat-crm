package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAddMonths(t *testing.T) {
	tests := []struct {
		name     string
		start    time.Time
		months   int
		expected time.Time
	}{
		{
			name:     "Jan 31 + 1 month = Feb 28 (non-leap year)",
			start:    time.Date(2023, 1, 31, 12, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2023, 2, 28, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Jan 31 + 1 month = Feb 29 (leap year)",
			start:    time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Jan 30 + 1 month = Feb 28 (non-leap year)",
			start:    time.Date(2023, 1, 30, 12, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2023, 2, 28, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Jan 29 + 1 month = Feb 28 (non-leap year)",
			start:    time.Date(2023, 1, 29, 12, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2023, 2, 28, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Jan 28 + 1 month = Feb 28 (no clamping needed)",
			start:    time.Date(2023, 1, 28, 12, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2023, 2, 28, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Jan 15 + 1 month = Feb 15 (normal case)",
			start:    time.Date(2023, 1, 15, 12, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2023, 2, 15, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Mar 31 + 1 month = Apr 30",
			start:    time.Date(2023, 3, 31, 12, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2023, 4, 30, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Aug 31 + 3 months (quarterly) = Nov 30",
			start:    time.Date(2023, 8, 31, 12, 0, 0, 0, time.UTC),
			months:   3,
			expected: time.Date(2023, 11, 30, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Aug 31 + 6 months = Feb 28",
			start:    time.Date(2023, 8, 31, 12, 0, 0, 0, time.UTC),
			months:   6,
			expected: time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC), // 2024 is a leap year
		},
		{
			name:     "Dec 31 + 1 month = Jan 31 (year rollover)",
			start:    time.Date(2023, 12, 31, 12, 0, 0, 0, time.UTC),
			months:   1,
			expected: time.Date(2024, 1, 31, 12, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addMonths(tt.start, tt.months)
			assert.Equal(t, tt.expected, result, "Expected %v but got %v", tt.expected, result)
		})
	}
}

func TestAddYears(t *testing.T) {
	tests := []struct {
		name     string
		start    time.Time
		years    int
		expected time.Time
	}{
		{
			name:     "Feb 29 2024 + 1 year = Feb 28 2025 (leap to non-leap)",
			start:    time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			years:    1,
			expected: time.Date(2025, 2, 28, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Feb 28 2023 + 1 year = Feb 28 2024 (normal case)",
			start:    time.Date(2023, 2, 28, 12, 0, 0, 0, time.UTC),
			years:    1,
			expected: time.Date(2024, 2, 28, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Feb 29 2024 + 4 years = Feb 29 2028 (leap to leap)",
			start:    time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
			years:    4,
			expected: time.Date(2028, 2, 29, 12, 0, 0, 0, time.UTC),
		},
		{
			name:     "Jan 15 2023 + 1 year = Jan 15 2024 (normal case)",
			start:    time.Date(2023, 1, 15, 12, 0, 0, 0, time.UTC),
			years:    1,
			expected: time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := addYears(tt.start, tt.years)
			assert.Equal(t, tt.expected, result, "Expected %v but got %v", tt.expected, result)
		})
	}
}

func TestCalculateNextReminderTimeMonthlyEdgeCases(t *testing.T) {
	reoccurFalse := false

	tests := []struct {
		name     string
		reminder Reminder
		expected time.Time
	}{
		{
			name: "Monthly from Jan 31 should go to Feb 28",
			reminder: Reminder{
				RemindAt:              time.Date(2023, 1, 31, 12, 0, 0, 0, time.UTC),
				Recurrence:            "monthly",
				ReoccurFromCompletion: &reoccurFalse,
			},
			expected: time.Date(2023, 2, 28, 12, 0, 0, 0, time.UTC),
		},
		{
			name: "Monthly from Mar 31 should go to Apr 30",
			reminder: Reminder{
				RemindAt:              time.Date(2023, 3, 31, 12, 0, 0, 0, time.UTC),
				Recurrence:            "monthly",
				ReoccurFromCompletion: &reoccurFalse,
			},
			expected: time.Date(2023, 4, 30, 12, 0, 0, 0, time.UTC),
		},
		{
			name: "Quarterly from Aug 31 should go to Nov 30",
			reminder: Reminder{
				RemindAt:              time.Date(2023, 8, 31, 12, 0, 0, 0, time.UTC),
				Recurrence:            "quarterly",
				ReoccurFromCompletion: &reoccurFalse,
			},
			expected: time.Date(2023, 11, 30, 12, 0, 0, 0, time.UTC),
		},
		{
			name: "Yearly from Feb 29 leap year should go to Feb 28 non-leap",
			reminder: Reminder{
				RemindAt:              time.Date(2024, 2, 29, 12, 0, 0, 0, time.UTC),
				Recurrence:            "yearly",
				ReoccurFromCompletion: &reoccurFalse,
			},
			expected: time.Date(2025, 2, 28, 12, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.reminder.NextOccurrence(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
			assert.Equal(t, tt.expected, result, "Expected %v but got %v", tt.expected, result)
		})
	}
}

func TestCalculateNextReminderTimeUTCConsistency(t *testing.T) {
	reoccurFalse := false

	// Create a reminder with a non-UTC timezone
	pst, _ := time.LoadLocation("America/Los_Angeles")
	remindAt := time.Date(2023, 1, 15, 9, 0, 0, 0, pst)

	reminder := Reminder{
		RemindAt:              remindAt,
		Recurrence:            "weekly",
		ReoccurFromCompletion: &reoccurFalse,
	}

	result := reminder.NextOccurrence(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	// Result should be in UTC
	assert.Equal(t, time.UTC, result.Location(), "Result should be in UTC")

	// Should be exactly 7 days later
	expectedUTC := remindAt.UTC().AddDate(0, 0, 7)
	assert.Equal(t, expectedUTC, result, "Should be 7 days after the original UTC time")
}

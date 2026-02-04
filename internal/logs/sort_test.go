package logs

import (
	"testing"
	"time"
)

func TestSortEvents(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name  string
		input []TailEvent
		want  []time.Time // expected timestamps in order
	}{
		{
			name:  "empty slice",
			input: nil,
			want:  nil,
		},
		{
			name: "single event",
			input: []TailEvent{
				{Timestamp: now, Message: "single"},
			},
			want: []time.Time{now},
		},
		{
			name: "already sorted",
			input: []TailEvent{
				{Timestamp: now.Add(-2 * time.Second), Message: "first"},
				{Timestamp: now.Add(-1 * time.Second), Message: "second"},
				{Timestamp: now, Message: "third"},
			},
			want: []time.Time{
				now.Add(-2 * time.Second),
				now.Add(-1 * time.Second),
				now,
			},
		},
		{
			name: "reverse order",
			input: []TailEvent{
				{Timestamp: now, Message: "third"},
				{Timestamp: now.Add(-1 * time.Second), Message: "second"},
				{Timestamp: now.Add(-2 * time.Second), Message: "first"},
			},
			want: []time.Time{
				now.Add(-2 * time.Second),
				now.Add(-1 * time.Second),
				now,
			},
		},
		{
			name: "random order",
			input: []TailEvent{
				{Timestamp: now.Add(-1 * time.Second), Message: "second"},
				{Timestamp: now.Add(-3 * time.Second), Message: "first"},
				{Timestamp: now, Message: "fourth"},
				{Timestamp: now.Add(-2 * time.Second), Message: "third"},
			},
			want: []time.Time{
				now.Add(-3 * time.Second),
				now.Add(-2 * time.Second),
				now.Add(-1 * time.Second),
				now,
			},
		},
		{
			name: "same timestamps preserved",
			input: []TailEvent{
				{Timestamp: now, Message: "a"},
				{Timestamp: now, Message: "b"},
				{Timestamp: now, Message: "c"},
			},
			want: []time.Time{now, now, now},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sortEvents(tt.input)

			if len(tt.input) != len(tt.want) {
				t.Fatalf("length mismatch: got %d, want %d", len(tt.input), len(tt.want))
			}

			for i, event := range tt.input {
				if !event.Timestamp.Equal(tt.want[i]) {
					t.Errorf("index %d: got %v, want %v", i, event.Timestamp, tt.want[i])
				}
			}
		})
	}
}

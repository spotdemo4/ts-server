package putil

import (
	"database/sql"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func Null[T any](a *T) *sql.Null[T] {
	if a == nil {
		n := sql.Null[T]{
			Valid: false,
		}
		return &n
	}

	n := sql.Null[T]{
		V:     *a,
		Valid: true,
	}
	return &n
}

func NullLike(s *string) *string {
	if s == nil {
		return nil
	}

	ts := fmt.Sprintf("%%%s%%", *s)
	return &ts
}

func NullTimestamp(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}

	t := ts.AsTime()
	return &t
}

func NullFloat64(f *float32) *float64 {
	if f == nil {
		return nil
	}

	f64 := float64(*f)
	return &f64
}

func NullInt64(i *int32) *int64 {
	if i == nil {
		return nil
	}

	i64 := int64(*i)
	return &i64
}

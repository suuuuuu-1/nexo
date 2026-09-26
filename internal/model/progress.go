package model

import "time"

type WatchProgress struct {
	// Version 用于乐观锁，避免多个设备同时上报进度时互相覆盖。
	UserID          string    `json:"user_id"`
	EpisodeID       string    `json:"episode_id"`
	PositionSeconds int       `json:"position_seconds"`
	DurationSeconds int       `json:"duration_seconds"`
	Version         int64     `json:"version"`
	LastWatchedAt   time.Time `json:"last_watched_at"`
}

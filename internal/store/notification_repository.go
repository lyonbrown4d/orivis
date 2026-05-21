package store

import (
	"context"
	"time"

	"github.com/arcgolabs/dbx"
	columnx "github.com/arcgolabs/dbx/column"
	"github.com/arcgolabs/dbx/querydsl"
	repository "github.com/arcgolabs/dbx/repository"
	schemax "github.com/arcgolabs/dbx/schema"
)

const (
	NotificationChannelWebhook = "webhook"
	NotificationStatusSuccess  = "success"
	NotificationStatusFailed   = "failed"
)

type NotificationDeliveryParams struct {
	Channel       string
	Event         string
	MonitorID     string
	AgentID       string
	RegionID      string
	EnvironmentID string
	Status        string
	Attempt       int
	MaxAttempts   int
	HTTPStatus    int
	Duration      time.Duration
	ErrorMessage  string
	CheckedAt     time.Time
	SentAt        time.Time
}

type DashboardNotification struct {
	ID            string
	Channel       string
	Event         string
	MonitorID     string
	AgentID       string
	RegionID      string
	EnvironmentID string
	Status        string
	Attempt       int
	MaxAttempts   int
	HTTPStatus    int
	Duration      time.Duration
	ErrorMessage  string
	CheckedAt     time.Time
	SentAt        time.Time
	CreatedAt     time.Time
}

type notificationDeliveryRow struct {
	ID            string `dbx:"id"`
	Channel       string `dbx:"channel"`
	Event         string `dbx:"event"`
	MonitorID     string `dbx:"monitor_id"`
	AgentID       string `dbx:"agent_id"`
	RegionID      string `dbx:"region_id"`
	EnvironmentID string `dbx:"environment_id"`
	Status        string `dbx:"status"`
	Attempt       int    `dbx:"attempt"`
	MaxAttempts   int    `dbx:"max_attempts"`
	HTTPStatus    int    `dbx:"http_status"`
	DurationMS    int64  `dbx:"duration_ms"`
	ErrorMessage  string `dbx:"error_message"`
	CheckedAt     string `dbx:"checked_at"`
	SentAt        string `dbx:"sent_at"`
	CreatedAt     string `dbx:"created_at"`
}

type notificationDeliverySchema struct {
	schemax.Schema[notificationDeliveryRow]
	ID            columnx.Column[notificationDeliveryRow, string] `dbx:"id,pk"`
	Channel       columnx.Column[notificationDeliveryRow, string] `dbx:"channel"`
	Event         columnx.Column[notificationDeliveryRow, string] `dbx:"event"`
	MonitorID     columnx.Column[notificationDeliveryRow, string] `dbx:"monitor_id"`
	AgentID       columnx.Column[notificationDeliveryRow, string] `dbx:"agent_id"`
	RegionID      columnx.Column[notificationDeliveryRow, string] `dbx:"region_id"`
	EnvironmentID columnx.Column[notificationDeliveryRow, string] `dbx:"environment_id"`
	Status        columnx.Column[notificationDeliveryRow, string] `dbx:"status"`
	Attempt       columnx.Column[notificationDeliveryRow, int]    `dbx:"attempt"`
	MaxAttempts   columnx.Column[notificationDeliveryRow, int]    `dbx:"max_attempts"`
	HTTPStatus    columnx.Column[notificationDeliveryRow, int]    `dbx:"http_status"`
	DurationMS    columnx.Column[notificationDeliveryRow, int64]  `dbx:"duration_ms"`
	ErrorMessage  columnx.Column[notificationDeliveryRow, string] `dbx:"error_message"`
	CheckedAt     columnx.Column[notificationDeliveryRow, string] `dbx:"checked_at"`
	SentAt        columnx.Column[notificationDeliveryRow, string] `dbx:"sent_at"`
	CreatedAt     columnx.Column[notificationDeliveryRow, string] `dbx:"created_at"`
}

func newNotificationDeliveryRepository(database *dbx.DB) *repository.Base[notificationDeliveryRow, notificationDeliverySchema] {
	return repository.New[notificationDeliveryRow](database, notificationDeliverySchemaResource())
}

func notificationDeliverySchemaResource() notificationDeliverySchema {
	return schemax.MustSchema("notification_deliveries", notificationDeliverySchema{})
}

func (s *Store) RecordNotificationDelivery(ctx context.Context, params NotificationDeliveryParams) error {
	if s == nil || s.repositories == nil || s.ids == nil {
		return nil
	}
	id, err := s.ids.NewID(ctx, "ntf")
	if err != nil {
		return wrapError(err, "new notification delivery id")
	}
	now := time.Now().UTC()
	row := notificationDeliveryRow{
		ID:            id,
		Channel:       params.Channel,
		Event:         params.Event,
		MonitorID:     params.MonitorID,
		AgentID:       params.AgentID,
		RegionID:      params.RegionID,
		EnvironmentID: params.EnvironmentID,
		Status:        params.Status,
		Attempt:       params.Attempt,
		MaxAttempts:   params.MaxAttempts,
		HTTPStatus:    params.HTTPStatus,
		DurationMS:    params.Duration.Milliseconds(),
		ErrorMessage:  params.ErrorMessage,
		CheckedAt:     formatTime(params.CheckedAt),
		SentAt:        formatTime(params.SentAt),
		CreatedAt:     formatTime(now),
	}
	if err := s.repositories.notificationDeliveries.Create(ctx, &row); err != nil {
		return wrapError(err, "record notification delivery")
	}
	return nil
}

func (s *Store) DashboardNotifications(ctx context.Context, limit int) ([]DashboardNotification, error) {
	if s == nil || s.repositories == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.repositories.notificationDeliveries.List(
		ctx,
		querydsl.Select(querydsl.AllColumns(notificationDeliverySchemaResource()).Values()...).
			From(notificationDeliverySchemaResource()).
			OrderBy(notificationDeliverySchemaResource().CreatedAt.Desc()).
			Limit(limit),
	)
	if err != nil {
		return nil, wrapError(err, "list dashboard notifications")
	}
	rowsValues := rows.Values()
	notifications := make([]DashboardNotification, 0, len(rowsValues))
	for index := range rowsValues {
		row := rowsValues[index]
		item, err := dashboardNotificationFromRow(row)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, item)
	}
	return notifications, nil
}

func dashboardNotificationFromRow(row notificationDeliveryRow) (DashboardNotification, error) {
	checkedAt, err := parseTime(row.CheckedAt)
	if err != nil {
		return DashboardNotification{}, err
	}
	sentAt, err := parseTime(row.SentAt)
	if err != nil {
		return DashboardNotification{}, err
	}
	createdAt, err := parseTime(row.CreatedAt)
	if err != nil {
		return DashboardNotification{}, err
	}
	return DashboardNotification{
		ID:            row.ID,
		Channel:       row.Channel,
		Event:         row.Event,
		MonitorID:     row.MonitorID,
		AgentID:       row.AgentID,
		RegionID:      row.RegionID,
		EnvironmentID: row.EnvironmentID,
		Status:        row.Status,
		Attempt:       row.Attempt,
		MaxAttempts:   row.MaxAttempts,
		HTTPStatus:    row.HTTPStatus,
		Duration:      time.Duration(row.DurationMS) * time.Millisecond,
		ErrorMessage:  row.ErrorMessage,
		CheckedAt:     checkedAt,
		SentAt:        sentAt,
		CreatedAt:     createdAt,
	}, nil
}

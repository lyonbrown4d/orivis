package notification

import (
	"encoding/json"
	"time"
)

func notificationDeliveryBody(delivery webhookDelivery) ([]byte, error) {
	switch notificationChannelType(delivery.channel.channelType) {
	case notificationChannelAlertmanager:
		payload := alertmanagerPayloadFromWebhookPayload(delivery.payload)
		body, err := json.Marshal([]alertmanagerPayload{payload})
		if err != nil {
			return nil, wrapError(err, "marshal alertmanager payload")
		}
		return body, nil
	default:
		body, err := json.Marshal(delivery.payload)
		if err != nil {
			return nil, wrapError(err, "marshal webhook payload")
		}
		return body, nil
	}
}

type alertmanagerPayload struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      *time.Time        `json:"endsAt,omitempty"`
	Status      string            `json:"status"`
}

func alertmanagerPayloadFromWebhookPayload(payload webhookPayload) alertmanagerPayload {
	started := payload.CheckedAt
	alert := alertmanagerPayload{
		Labels: map[string]string{
			"alertname":       "orivis_monitor",
			"channel":         payload.Channel,
			"monitor_id":      payload.MonitorID,
			"agent_id":        payload.AgentID,
			"region_id":       payload.RegionID,
			"environment_id":  payload.EnvironmentID,
			"check_status":    string(payload.Status),
			"event":           payload.Event,
			"status":          string(payload.Status),
			"resolved":        boolToString(payload.Event == "monitor_recovered"),
			"severity":        alertmanagerSeverity(payload.Event),
			"orivis_instance": "server",
		},
		Annotations: map[string]string{
			"summary":     "Orivis monitor " + payload.MonitorID,
			"description": alertmanagerDescription(payload),
		},
		StartsAt: started,
		Status:   alertStatus(payload.Event),
	}
	if alertStatus(payload.Event) == "resolved" {
		alert.EndsAt = &started
	}
	return alert
}

func alertStatus(event string) string {
	if event == "monitor_recovered" {
		return "resolved"
	}
	return "firing"
}

func alertmanagerSeverity(event string) string {
	if event == "monitor_recovered" {
		return "info"
	}
	return "critical"
}

func alertmanagerDescription(payload webhookPayload) string {
	description := "Monitor " + payload.MonitorID + " event " + payload.Event
	if payload.ErrorMessage != "" {
		description += ": " + payload.ErrorMessage
	}
	return description
}

func boolToString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

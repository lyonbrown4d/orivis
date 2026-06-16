package notification

import (
	"net/http"
	"strings"

	config "github.com/lyonbrown4d/orivis/internal/serverconfig"
	"github.com/samber/lo"
	"github.com/samber/mo"
)

type webhookChannel struct {
	channelType string
	name        string
	url         string
	method      string
	secret      string
	headers     []string
	monitorIDs  []string
	groups      []string
}

func webhookChannelsFromConfig(cfg config.Config) ([]webhookChannel, error) {
	channels := make([]webhookChannel, 0, len(cfg.Notification.Webhook.Routes)+1)
	if cfg.Notification.Webhook.Enabled && strings.TrimSpace(cfg.Notification.Webhook.URL) != "" {
		channels = append(channels, defaultWebhookChannel(cfg))
	}
	for _, entry := range cfg.Notification.Webhook.Routes {
		channel, err := parseWebhookRoute(entry, cfg)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(channel.url) != "" {
			channels = append(channels, channel)
		}
	}
	return channels, nil
}

func defaultWebhookChannel(cfg config.Config) webhookChannel {
	return webhookChannel{
		channelType: notificationChannelWebhook,
		name:        "webhook",
		url:         strings.TrimSpace(cfg.Notification.Webhook.URL),
		method:      webhookMethod(cfg.Notification.Webhook.Method),
		secret:      cfg.Notification.Webhook.Secret,
		headers:     cfg.Notification.Webhook.Headers,
	}
}

func parseWebhookRoute(entry string, cfg config.Config) (webhookChannel, error) {
	fields := parseRouteFields(entry)
	channel := webhookChannel{
		channelType: firstNonEmpty(fields["type"], notificationChannelWebhook),
		name:        firstNonEmpty(fields["name"], "webhook"),
		url:         strings.TrimSpace(fields["url"]),
		method:      webhookMethod(firstNonEmpty(fields["method"], cfg.Notification.Webhook.Method)),
		secret:      firstNonEmpty(fields["secret"], cfg.Notification.Webhook.Secret),
		headers:     routeList(firstNonEmpty(fields["headers"], strings.Join(cfg.Notification.Webhook.Headers, "|"))),
		monitorIDs:  routeList(fields["monitors"]),
		groups:      routeList(fields["groups"]),
	}
	if channel.url == "" {
		return webhookChannel{}, newErrorf("webhook route %q missing url", entry)
	}
	channel.channelType = normalizeNotificationChannelType(channel.channelType)
	if channel.channelType == "" {
		return webhookChannel{}, newErrorf("webhook route %q has unsupported type", entry)
	}
	return channel, nil
}

const notificationChannelWebhook = "webhook"

const notificationChannelAlertmanager = "alertmanager"

func parseRouteFields(entry string) map[string]string {
	fields := make(map[string]string)
	for part := range strings.SplitSeq(entry, ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if key != "" {
			fields[key] = value
		}
	}
	return fields
}

func routeList(value string) []string {
	out := make([]string, 0)
	for part := range strings.SplitSeq(value, "|") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (c webhookChannel) matches(monitorID, groupName string) bool {
	if len(c.monitorIDs) == 0 && len(c.groups) == 0 {
		return true
	}
	return containsFold(c.monitorIDs, monitorID) || containsFold(c.groups, groupName)
}

func matchingWebhookChannels(channels []webhookChannel, monitorID, groupName string) ([]webhookChannel, []webhookChannel) {
	matched := make([]webhookChannel, 0, len(channels))
	unmatched := make([]webhookChannel, 0, len(channels))
	for i := range channels {
		channel := channels[i]
		if channel.matches(monitorID, groupName) {
			matched = append(matched, channel)
			continue
		}
		unmatched = append(unmatched, channel)
	}
	return matched, unmatched
}

func (c webhookChannel) channelName() string {
	name := strings.TrimSpace(c.name)
	channelType := notificationChannelType(c.channelType)
	if name == "" || strings.EqualFold(name, channelType) {
		return channelType
	}
	return channelType + ":" + name
}

func notificationChannelType(rawType string) string {
	switch strings.ToLower(strings.TrimSpace(rawType)) {
	case notificationChannelAlertmanager:
		return notificationChannelAlertmanager
	default:
		return notificationChannelWebhook
	}
}

func normalizeNotificationChannelType(rawType string) string {
	channelType := notificationChannelType(rawType)
	if channelType == notificationChannelWebhook && rawType != "" && !strings.EqualFold(rawType, notificationChannelWebhook) {
		return ""
	}
	return channelType
}

func containsFold(values []string, target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	return lo.ContainsBy(values, func(value string) bool {
		return strings.EqualFold(value, target)
	})
}

func webhookMethod(method string) string {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return http.MethodPost
	}
	return method
}

func firstNonEmpty(values ...string) string {
	return mo.TupleToOption(lo.Find(values, func(value string) bool {
		return strings.TrimSpace(value) != ""
	})).OrEmpty()
}

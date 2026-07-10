package panel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tanselxy/singbox/internal/model"
	"github.com/tanselxy/singbox/internal/store"
)

const (
	notificationSettingKey = "notifications"
	notificationInterval   = 30 * time.Second
)

type notificationConfig struct {
	TelegramEnabled  bool   `json:"telegram_enabled"`
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramChatID   string `json:"telegram_chat_id"`

	NotifyNodeOffline  bool    `json:"notify_node_offline"`
	OfflineRepeatHours int     `json:"offline_repeat_hours"`
	NotifyCPU          bool    `json:"notify_cpu"`
	CPUThreshold       float64 `json:"cpu_threshold"`
	NotifyMemory       bool    `json:"notify_memory"`
	MemoryThreshold    float64 `json:"memory_threshold"`
	NotifyDisk         bool    `json:"notify_disk"`
	DiskThreshold      float64 `json:"disk_threshold"`

	CooldownMinutes int `json:"cooldown_minutes"`
}

type offlineNotificationState struct {
	Offline  bool
	Since    time.Time
	LastSent time.Time
}

func defaultNotificationConfig() notificationConfig {
	return notificationConfig{
		NotifyNodeOffline:  true,
		OfflineRepeatHours: 0,
		NotifyCPU:          true,
		CPUThreshold:       85,
		NotifyMemory:       true,
		MemoryThreshold:    85,
		NotifyDisk:         true,
		DiskThreshold:      90,
		CooldownMinutes:    30,
	}
}

func (s *Server) loadNotificationConfig() (notificationConfig, error) {
	cfg := defaultNotificationConfig()
	raw, err := s.db.GetSetting(notificationSettingKey)
	if errors.Is(err, store.ErrNotFound) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return defaultNotificationConfig(), nil
	}
	return sanitizeNotificationConfig(cfg), nil
}

func (s *Server) saveNotificationConfig(cfg notificationConfig) error {
	cfg = sanitizeNotificationConfig(cfg)
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return s.db.SetSetting(notificationSettingKey, string(b))
}

func sanitizeNotificationConfig(cfg notificationConfig) notificationConfig {
	cfg.TelegramBotToken = strings.TrimSpace(cfg.TelegramBotToken)
	cfg.TelegramChatID = strings.TrimSpace(cfg.TelegramChatID)
	cfg.CPUThreshold = clampThreshold(cfg.CPUThreshold, 85)
	cfg.MemoryThreshold = clampThreshold(cfg.MemoryThreshold, 85)
	cfg.DiskThreshold = clampThreshold(cfg.DiskThreshold, 90)
	if cfg.OfflineRepeatHours < 0 {
		cfg.OfflineRepeatHours = 0
	}
	if cfg.OfflineRepeatHours > 720 {
		cfg.OfflineRepeatHours = 720
	}
	if cfg.CooldownMinutes <= 0 {
		cfg.CooldownMinutes = 30
	}
	if cfg.CooldownMinutes > 1440 {
		cfg.CooldownMinutes = 1440
	}
	return cfg
}

func clampThreshold(value, fallback float64) float64 {
	if value <= 0 || value > 100 {
		return fallback
	}
	return value
}

func (s *Server) handleNotificationsGet(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.loadNotificationConfig()
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "config": cfg})
}

func (s *Server) handleNotificationsSave(w http.ResponseWriter, r *http.Request) {
	var cfg notificationConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "bad request"})
		return
	}
	if err := s.saveNotificationConfig(cfg); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleNotificationsTest(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.loadNotificationConfig()
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err := sendTelegram(r.Context(), cfg, "Sing-Box 面板测试通知：Telegram 通道已连通。"); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) runNotificationPoller(ctx context.Context) {
	ticker := time.NewTicker(notificationInterval)
	defer ticker.Stop()
	for {
		s.checkNotifications(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Server) checkNotifications(ctx context.Context) {
	cfg, err := s.loadNotificationConfig()
	if err != nil {
		return
	}
	telegramReady := cfg.TelegramEnabled && cfg.TelegramBotToken != "" && cfg.TelegramChatID != ""
	now := time.Now()
	s.checkNodeBilling(ctx, cfg, telegramReady, now)
	if !telegramReady {
		return
	}
	rows, err := s.nodeRows(ctx)
	if err != nil {
		return
	}
	cooldown := time.Duration(cfg.CooldownMinutes) * time.Minute
	offlineRepeat := time.Duration(cfg.OfflineRepeatHours) * time.Hour
	for _, row := range rows {
		// Freshly added nodes have no sample until the metrics poller completes.
		// Do not treat that brief bootstrap period as an offline event.
		if row.Pending {
			continue
		}
		var reasons []string
		if !row.Online {
			if cfg.NotifyNodeOffline {
				if reason, due := s.markOfflineNotification(row.ID, offlineRepeat, time.Now()); due {
					_ = sendTelegram(ctx, cfg, notificationMessage(row.Name, row.Address, []string{reason}))
				}
			}
			continue
		}
		if cfg.NotifyNodeOffline && s.markRecoveredNotification(row.ID) {
			reasons = append(reasons, "节点已恢复")
		}
		if !cfg.NotifyNodeOffline {
			s.clearOfflineNotification(row.ID)
		}
		if cfg.NotifyCPU && row.CPUPercent >= cfg.CPUThreshold && s.markNotificationDue(row.ID, "cpu", cooldown) {
			reasons = append(reasons, fmt.Sprintf("CPU %.1f%% >= %.1f%%", row.CPUPercent, cfg.CPUThreshold))
		}
		if cfg.NotifyMemory && row.MemPercent >= cfg.MemoryThreshold && s.markNotificationDue(row.ID, "memory", cooldown) {
			reasons = append(reasons, fmt.Sprintf("内存 %.1f%% >= %.1f%%", row.MemPercent, cfg.MemoryThreshold))
		}
		if cfg.NotifyDisk && row.DiskPercent >= cfg.DiskThreshold && s.markNotificationDue(row.ID, "disk", cooldown) {
			reasons = append(reasons, fmt.Sprintf("硬盘 %.1f%% >= %.1f%%", row.DiskPercent, cfg.DiskThreshold))
		}
		if len(reasons) > 0 {
			_ = sendTelegram(ctx, cfg, notificationMessage(row.Name, row.Address, reasons))
		}
	}
}

func (s *Server) checkNodeBilling(ctx context.Context, cfg notificationConfig, telegramReady bool, now time.Time) {
	nodes, err := s.db.ListNodes()
	if err != nil {
		return
	}
	for _, node := range nodes {
		updated := false
		for node.EndAt > 0 && now.Unix() >= node.EndAt && normalizeNodeBillingCycle(node.BillingCycle) != "" {
			nextStart := node.EndAt
			nextEnd := addNodeBillingCycle(time.Unix(nextStart, 0).UTC(), node.BillingCycle).Unix()
			if nextEnd <= node.EndAt {
				break
			}
			node.StartAt = nextStart
			node.EndAt = nextEnd
			node.NextRemindAt = 0
			updated = true
		}
		if updated {
			_ = s.db.UpdateNodeBilling(node.ID, node.StartAt, node.EndAt, node.NextRemindAt)
		}
		if !telegramReady || !nodeRenewalReminderDue(node, now) {
			continue
		}
		_ = sendTelegram(ctx, cfg, nodeRenewalMessage(node, now))
		nextRemindAt := nextNodeRenewalReminderAt(now)
		if nextRemindAt >= node.EndAt {
			nextRemindAt = node.EndAt
		}
		_ = s.db.UpdateNodeBilling(node.ID, node.StartAt, node.EndAt, nextRemindAt)
	}
}

func addNodeBillingCycle(start time.Time, cycle string) time.Time {
	switch normalizeNodeBillingCycle(cycle) {
	case "monthly":
		return start.AddDate(0, 1, 0)
	case "quarterly":
		return start.AddDate(0, 3, 0)
	case "half_year":
		return start.AddDate(0, 6, 0)
	case "yearly":
		return start.AddDate(1, 0, 0)
	case "two_years":
		return start.AddDate(2, 0, 0)
	case "three_years":
		return start.AddDate(3, 0, 0)
	default:
		return start
	}
}

func nodeRenewalReminderDue(node model.Node, now time.Time) bool {
	if node.EndAt <= 0 || normalizeNodeBillingCycle(node.BillingCycle) == "" {
		return false
	}
	nowUnix := now.Unix()
	if nowUnix >= node.EndAt || nowUnix < node.EndAt-int64((7*24*time.Hour)/time.Second) {
		return false
	}
	return node.NextRemindAt <= 0 || nowUnix >= node.NextRemindAt
}

func nextNodeRenewalReminderAt(now time.Time) int64 {
	local := now.In(time.Local)
	next := time.Date(local.Year(), local.Month(), local.Day()+1, 9, 0, 0, 0, time.Local)
	return next.Unix()
}

func nodeRenewalMessage(node model.Node, now time.Time) string {
	daySeconds := int64((24 * time.Hour) / time.Second)
	days := int((node.EndAt - now.Unix() + daySeconds - 1) / daySeconds)
	if days < 0 {
		days = 0
	}
	return fmt.Sprintf(
		"Sing-Box 机器续费提醒\n节点：%s\n地址：%s\n续费日期：%s\n周期：%s\n内容：距离当前周期结束还有 %d 天，请及时确认续费。",
		node.Name, node.Address, nodeDateInput(node.EndAt), nodeBillingCycleLabel(node.BillingCycle), days,
	)
}

func (s *Server) markNotificationDue(nodeID int64, event string, cooldown time.Duration) bool {
	key := fmt.Sprintf("%d:%s", nodeID, event)
	now := time.Now()
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()
	if last, ok := s.notifyLast[key]; ok && now.Sub(last) < cooldown {
		return false
	}
	s.notifyLast[key] = now
	return true
}

func (s *Server) markOfflineNotification(nodeID int64, repeat time.Duration, now time.Time) (string, bool) {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()
	if s.notifyOffline == nil {
		s.notifyOffline = map[int64]offlineNotificationState{}
	}
	state, ok := s.notifyOffline[nodeID]
	if !ok || !state.Offline {
		s.notifyOffline[nodeID] = offlineNotificationState{Offline: true, Since: now, LastSent: now}
		return "节点离线", true
	}
	if repeat > 0 && now.Sub(state.LastSent) >= repeat {
		state.LastSent = now
		s.notifyOffline[nodeID] = state
		return fmt.Sprintf("节点持续离线（已持续 %s）", formatOfflineDuration(now.Sub(state.Since))), true
	}
	return "", false
}

func (s *Server) markRecoveredNotification(nodeID int64) bool {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()
	state, ok := s.notifyOffline[nodeID]
	if !ok || !state.Offline {
		return false
	}
	delete(s.notifyOffline, nodeID)
	return true
}

func (s *Server) clearOfflineNotification(nodeID int64) {
	s.notifyMu.Lock()
	defer s.notifyMu.Unlock()
	delete(s.notifyOffline, nodeID)
}

func formatOfflineDuration(d time.Duration) string {
	if d < time.Minute {
		return "不足 1 分钟"
	}
	hours := int(d / time.Hour)
	minutes := int((d % time.Hour) / time.Minute)
	if hours > 0 && minutes > 0 {
		return fmt.Sprintf("%d 小时 %d 分钟", hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%d 小时", hours)
	}
	return fmt.Sprintf("%d 分钟", minutes)
}

func notificationMessage(name, address string, reasons []string) string {
	return fmt.Sprintf("Sing-Box 节点告警\n节点：%s\n地址：%s\n内容：%s", name, address, strings.Join(reasons, "；"))
}

func sendTelegram(ctx context.Context, cfg notificationConfig, text string) error {
	if cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		return fmt.Errorf("请先填写 Telegram Bot Token 和 Chat ID")
	}
	form := url.Values{}
	form.Set("chat_id", cfg.TelegramChatID)
	form.Set("text", text)
	form.Set("disable_web_page_preview", "true")
	endpoint := "https://api.telegram.org/bot" + cfg.TelegramBotToken + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Telegram 返回状态 %d", resp.StatusCode)
	}
	return nil
}

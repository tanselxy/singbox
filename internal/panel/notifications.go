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

	NotifyNodeOffline bool    `json:"notify_node_offline"`
	NotifyCPU         bool    `json:"notify_cpu"`
	CPUThreshold      float64 `json:"cpu_threshold"`
	NotifyMemory      bool    `json:"notify_memory"`
	MemoryThreshold   float64 `json:"memory_threshold"`
	NotifyDisk        bool    `json:"notify_disk"`
	DiskThreshold     float64 `json:"disk_threshold"`

	CooldownMinutes int `json:"cooldown_minutes"`
}

func defaultNotificationConfig() notificationConfig {
	return notificationConfig{
		NotifyNodeOffline: true,
		NotifyCPU:         true,
		CPUThreshold:      85,
		NotifyMemory:      true,
		MemoryThreshold:   85,
		NotifyDisk:        true,
		DiskThreshold:     90,
		CooldownMinutes:   30,
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
	if err != nil || !cfg.TelegramEnabled || cfg.TelegramBotToken == "" || cfg.TelegramChatID == "" {
		return
	}
	rows, err := s.nodeRows(ctx)
	if err != nil {
		return
	}
	cooldown := time.Duration(cfg.CooldownMinutes) * time.Minute
	for _, row := range rows {
		var reasons []string
		if cfg.NotifyNodeOffline && !row.Online && s.markNotificationDue(row.ID, "offline", cooldown) {
			reasons = append(reasons, "节点离线")
		}
		if !row.Online {
			if len(reasons) > 0 {
				_ = sendTelegram(ctx, cfg, notificationMessage(row.Name, row.Address, reasons))
			}
			continue
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

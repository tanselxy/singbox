package panel

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tanselxy/singbox/internal/config"
	"github.com/tanselxy/singbox/internal/model"
)

type clashConnectionsResponse struct {
	Connections []clashConnection `json:"connections"`
}

type clashConnection struct {
	ID       string                  `json:"id"`
	Metadata clashConnectionMetadata `json:"metadata"`
	Upload   int64                   `json:"upload"`
	Download int64                   `json:"download"`
	Start    time.Time               `json:"start"`
}

type clashConnectionMetadata struct {
	User       string `json:"user"`
	Network    string `json:"network"`
	Type       string `json:"type"`
	SourceIP   string `json:"sourceIP"`
	SourcePort string `json:"sourcePort"`
	Host       string `json:"host"`
}

type clientDeviceRow struct {
	SourceIP      string   `json:"source_ip"`
	Connections   int      `json:"connections"`
	Current       string   `json:"current"`
	Upload        string   `json:"upload"`
	Download      string   `json:"download"`
	CurrentBytes  int64    `json:"current_bytes"`
	UploadBytes   int64    `json:"upload_bytes"`
	DownloadBytes int64    `json:"download_bytes"`
	FirstSeen     string   `json:"first_seen"`
	LastSeen      string   `json:"last_seen"`
	Protocols     []string `json:"protocols"`
}

func (s *Server) handleClientDevices(w http.ResponseWriter, r *http.Request) {
	c, err := s.clientFromPath(r)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	connections, err := fetchClashConnections(r)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	tr, _ := s.db.GetTraffic(c.ID)
	used := tr.Up + tr.Down
	writeJSON(w, map[string]any{
		"ok": true,
		"client": map[string]any{
			"used":        humanBytes(used),
			"quota":       quotaLabel(c.QuotaBytes),
			"used_bytes":  used,
			"quota_bytes": c.QuotaBytes,
		},
		"devices": groupClientDevices(connections, c.Name, c.QuotaBytes),
	})
}

func (s *Server) handleClientDeviceKick(w http.ResponseWriter, r *http.Request) {
	c, err := s.clientFromPath(r)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err := r.ParseForm(); err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": "解析表单失败: " + err.Error()})
		return
	}
	sourceIP := strings.TrimSpace(r.PostFormValue("source_ip"))
	if sourceIP == "" {
		writeJSON(w, map[string]any{"ok": false, "error": "缺少设备 IP"})
		return
	}
	connections, err := fetchClashConnections(r)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	closed := 0
	for _, conn := range connections {
		if conn.Metadata.User != c.Name || conn.Metadata.SourceIP != sourceIP || conn.ID == "" {
			continue
		}
		if err := closeClashConnection(r, conn.ID); err != nil {
			writeJSON(w, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		closed++
	}
	writeJSON(w, map[string]any{"ok": true, "closed": closed})
}

func (s *Server) clientFromPath(r *http.Request) (model.Client, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return model.Client{}, fmt.Errorf("客户 ID 无效")
	}
	return s.db.GetClient(id)
}

func fetchClashConnections(r *http.Request) ([]clashConnection, error) {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://"+config.ClashAPIAddr+"/connections", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.ClashAPISecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("读取设备连接失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取设备连接失败: clash api 状态 %d", resp.StatusCode)
	}
	var payload clashConnectionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("解析设备连接失败: %w", err)
	}
	return payload.Connections, nil
}

func closeClashConnection(r *http.Request, id string) error {
	req, err := http.NewRequestWithContext(r.Context(), http.MethodDelete, "http://"+config.ClashAPIAddr+"/connections/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+config.ClashAPISecret)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("踢下线失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("踢下线失败: clash api 状态 %d", resp.StatusCode)
	}
	return nil
}

func groupClientDevices(connections []clashConnection, user string, quotaBytes int64) []clientDeviceRow {
	type aggregate struct {
		row      clientDeviceRow
		first    time.Time
		last     time.Time
		protocol map[string]bool
	}
	groups := map[string]*aggregate{}
	for _, conn := range connections {
		if conn.Metadata.User != user || conn.Metadata.SourceIP == "" {
			continue
		}
		group := groups[conn.Metadata.SourceIP]
		if group == nil {
			group = &aggregate{protocol: map[string]bool{}}
			group.row.SourceIP = conn.Metadata.SourceIP
			groups[conn.Metadata.SourceIP] = group
		}
		group.row.Connections++
		group.row.UploadBytes += conn.Upload
		group.row.DownloadBytes += conn.Download
		if conn.Metadata.Type != "" {
			group.protocol[conn.Metadata.Type] = true
		}
		if !conn.Start.IsZero() {
			if group.first.IsZero() || conn.Start.Before(group.first) {
				group.first = conn.Start
			}
			if conn.Start.After(group.last) {
				group.last = conn.Start
			}
		}
	}
	devices := make([]clientDeviceRow, 0, len(groups))
	for _, group := range groups {
		group.row.CurrentBytes = group.row.UploadBytes + group.row.DownloadBytes
		if quotaBytes > 0 {
			group.row.Current = humanBytes(group.row.CurrentBytes) + " / " + humanBytes(quotaBytes)
		} else {
			group.row.Current = humanBytes(group.row.CurrentBytes) + " / 不限"
		}
		group.row.Upload = humanBytes(group.row.UploadBytes)
		group.row.Download = humanBytes(group.row.DownloadBytes)
		group.row.FirstSeen = formatDeviceTime(group.first)
		group.row.LastSeen = formatDeviceTime(group.last)
		for protocol := range group.protocol {
			group.row.Protocols = append(group.row.Protocols, protocol)
		}
		sort.Strings(group.row.Protocols)
		devices = append(devices, group.row)
	}
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].LastSeen > devices[j].LastSeen
	})
	return devices
}

func formatDeviceTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

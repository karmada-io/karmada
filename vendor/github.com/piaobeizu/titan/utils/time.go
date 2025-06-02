/*
 * @Version : 1.0
 * @Author  : wangxiaokang
 * @Email   : xiaokang.w@gmicloud.ai
 * @Date    : 2025/06/01
 * @Desc    : time utils
 */

package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// TimeResponse 服务器时间响应
type TimeResponse struct {
	ServerUTC     string `json:"server_utc"`
	ServerLocal   string `json:"server_local"`
	Timestamp     int64  `json:"timestamp"`
	Timezone      string `json:"timezone"`
	RequestID     string `json:"request_id"`
	ServerVersion string `json:"server_version"`
	ProcessTime   string `json:"process_time"`
}

// ClientConfig 客户端配置
type ClientConfig struct {
	ServerURL    string        `json:"server_url"`
	ClientID     string        `json:"client_id"`
	Region       string        `json:"region"`
	Timezone     string        `json:"timezone"`
	SyncInterval time.Duration `json:"sync_interval"`
	Timeout      time.Duration `json:"timeout"`
	MaxRetries   int           `json:"max_retries"`
	EnableTLS    bool          `json:"enable_tls"`
}

// TimeClient 时间同步客户端
type TimeClient struct {
	config     *ClientConfig
	httpClient *http.Client
	localTZ    *time.Location
	running    bool
	stopCh     chan struct{}
}

// NewTimeClient 创建时间客户端
func NewTimeClient(config *ClientConfig) (*TimeClient, error) {
	// 加载本地时区
	localTZ, err := time.LoadLocation(config.Timezone)
	if err != nil {
		logrus.Printf("无法加载时区 %s，使用系统默认时区: %v", config.Timezone, err)
		localTZ = time.Local
	}

	client := &TimeClient{
		config:  config,
		localTZ: localTZ,
		stopCh:  make(chan struct{}),
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}

	return client, nil
}

// SyncTime 执行一次时间同步
func (tc *TimeClient) SyncTime() (*TimeSyncResult, error) {
	logrus.Printf("开始时间同步 [Client: %s, Region: %s]", tc.config.ClientID, tc.config.Region)

	// 记录请求开始时间
	t1 := time.Now()

	// 创建请求
	req, err := http.NewRequest("POST", tc.config.ServerURL+"/time/sync", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Client-ID", tc.config.ClientID)
	req.Header.Set("Region", tc.config.Region)
	req.Header.Set("Timezone", tc.config.Timezone)
	req.Header.Set("User-Agent", "TimeClient/1.0")

	// 发送请求
	resp, err := tc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// 记录响应接收时间
	t4 := time.Now()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server error %d: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var timeResp TimeResponse
	if err := json.NewDecoder(resp.Body).Decode(&timeResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// 解析服务器UTC时间
	serverTime, err := time.Parse("2006-01-02T15:04:05.000Z", timeResp.ServerUTC)
	if err != nil {
		return nil, fmt.Errorf("failed to parse server time: %v", err)
	}

	// 计算网络延迟和时间偏移
	networkDelay := t4.Sub(t1)
	estimatedServerTime := serverTime.Add(networkDelay / 2)
	offset := estimatedServerTime.Sub(t1)

	// 转换到本地时区
	localServerTime := serverTime.In(tc.localTZ)
	localCurrentTime := time.Now().In(tc.localTZ)

	result := &TimeSyncResult{
		Success:         true,
		ServerTimeUTC:   serverTime,
		ServerTimeLocal: localServerTime,
		ClientTimeLocal: localCurrentTime,
		NetworkDelay:    networkDelay,
		TimeOffset:      offset,
		RequestID:       timeResp.RequestID,
		SyncTimestamp:   time.Now(),
		ServerVersion:   timeResp.ServerVersion,
	}

	// 记录同步结果
	tc.logSyncResult(result)

	return result, nil
}

// TimeSyncResult 时间同步结果
type TimeSyncResult struct {
	Success         bool          `json:"success"`
	ServerTimeUTC   time.Time     `json:"server_time_utc"`
	ServerTimeLocal time.Time     `json:"server_time_local"`
	ClientTimeLocal time.Time     `json:"client_time_local"`
	NetworkDelay    time.Duration `json:"network_delay"`
	TimeOffset      time.Duration `json:"time_offset"`
	RequestID       string        `json:"request_id"`
	SyncTimestamp   time.Time     `json:"sync_timestamp"`
	ServerVersion   string        `json:"server_version"`
	Error           string        `json:"error,omitempty"`
}

// logSyncResult 记录同步结果
func (tc *TimeClient) logSyncResult(result *TimeSyncResult) {
	absOffset := result.TimeOffset
	if absOffset < 0 {
		absOffset = -absOffset
	}

	status := "SUCCESS"
	if absOffset > 5*time.Second {
		status = "WARNING"
	} else if absOffset > 30*time.Second {
		status = "ERROR"
	}

	logrus.Printf("[%s] 时间同步完成", status)
	logrus.Printf("   服务器UTC: %s", result.ServerTimeUTC.Format("2006-01-02 15:04:05.000"))
	logrus.Printf("   服务器本地: %s", result.ServerTimeLocal.Format("2006-01-02 15:04:05.000 MST"))
	logrus.Printf("   客户端本地: %s", result.ClientTimeLocal.Format("2006-01-02 15:04:05.000 MST"))
	logrus.Printf("   网络延迟: %v", result.NetworkDelay)
	logrus.Printf("   时间偏移: %v", result.TimeOffset)
	logrus.Printf("   请求ID: %s", result.RequestID)
}

// StartDaemon 启动守护进程模式
func (tc *TimeClient) StartDaemon() {
	logrus.Printf("启动时间同步守护进程")
	logrus.Printf("   同步间隔: %v", tc.config.SyncInterval)
	logrus.Printf("   服务器: %s", tc.config.ServerURL)
	logrus.Printf("   客户端ID: %s", tc.config.ClientID)
	logrus.Printf("   区域: %s", tc.config.Region)
	logrus.Printf("   时区: %s", tc.config.Timezone)

	tc.running = true
	ticker := time.NewTicker(tc.config.SyncInterval)
	defer ticker.Stop()

	// 设置信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 立即执行一次同步
	tc.syncWithRetry()

	for {
		select {
		case <-ticker.C:
			tc.syncWithRetry()
		case <-tc.stopCh:
			logrus.Println("收到停止信号，正在关闭守护进程...")
			tc.running = false
			return
		case sig := <-sigCh:
			logrus.Printf("收到系统信号 %v，正在关闭守护进程...", sig)
			tc.running = false
			return
		}
	}
}

// syncWithRetry 带重试的同步
func (tc *TimeClient) syncWithRetry() {
	for i := range tc.config.MaxRetries {
		result, err := tc.SyncTime()
		if err != nil {
			logrus.Printf("同步失败 (第%d次尝试): %v", i+1, err)
			if i < tc.config.MaxRetries-1 {
				backoff := time.Duration(i+1) * time.Second
				logrus.Printf("等待 %v 后重试...", backoff)
				time.Sleep(backoff)
			}
			continue
		}

		// 同步成功，检查是否需要调整系统时间
		if tc.shouldAdjustSystemTime(result) {
			tc.adjustSystemTime(result)
		}
		return
	}

	logrus.Printf("所有重试均失败，等待下次同步...")
}

// shouldAdjustSystemTime 判断是否需要调整系统时间
func (tc *TimeClient) shouldAdjustSystemTime(result *TimeSyncResult) bool {
	absOffset := result.TimeOffset
	if absOffset < 0 {
		absOffset = -absOffset
	}
	// 偏移超过5秒才调整
	return absOffset > 5*time.Second
}

// adjustSystemTime 调整系统时间（需要权限）
func (tc *TimeClient) adjustSystemTime(result *TimeSyncResult) {
	logrus.Printf("检测到时间偏移 %v，尝试调整系统时间...", result.TimeOffset)

	// 计算目标时间（服务器时间加上网络延迟补偿）
	targetTime := result.ServerTimeUTC.Add(result.NetworkDelay / 2)

	// 检查权限
	if !tc.hasAdminPermission() {
		logrus.Printf("需要管理员权限来调整系统时间")
		tc.showPermissionHelp()
		return
	}

	// 执行系统时间调整
	if err := tc.setSystemTime(targetTime); err != nil {
		logrus.Printf("系统时间调整失败: %v", err)

		// 尝试备用方法
		if err2 := tc.setSystemTimeAlternative(targetTime); err2 != nil {
			logrus.Printf("备用方法也失败: %v", err2)
			logrus.Printf("建议: 请检查权限或手动配置时间同步服务")
		}
		return
	}

	logrus.Printf("系统时间调整成功")
	logrus.Printf("   目标时间: %s", targetTime.Format("2006-01-02 15:04:05.000 UTC"))
	logrus.Printf("   时间偏移: %v", result.TimeOffset)

	// 验证调整结果
	tc.verifyTimeAdjustment(targetTime)
}

// Stop 停止客户端
func (tc *TimeClient) Stop() {
	if tc.running {
		close(tc.stopCh)
	}
}

// hasAdminPermission 检查是否有管理员权限
func (tc *TimeClient) hasAdminPermission() bool {
	switch runtime.GOOS {
	case "linux", "darwin":
		return os.Geteuid() == 0
	case "windows":
		// Windows权限检查
		cmd := exec.Command("net", "session")
		return cmd.Run() == nil
	default:
		return false
	}
}

// showPermissionHelp 显示权限帮助信息
func (tc *TimeClient) showPermissionHelp() {
	logrus.Printf("权限要求:")
	switch runtime.GOOS {
	case "linux":
		logrus.Printf("   Linux: sudo运行或以root用户运行")
		logrus.Printf("   命令示例: sudo ./time-client")
	case "darwin":
		logrus.Printf("   macOS: sudo运行或以管理员身份运行")
		logrus.Printf("   命令示例: sudo ./time-client")
	case "windows":
		logrus.Printf("   Windows: 以管理员身份运行")
		logrus.Printf("   右键程序 -> 以管理员身份运行")
	default:
		logrus.Printf("   请以管理员权限运行此程序")
	}
}

// setSystemTime 设置系统时间
func (tc *TimeClient) setSystemTime(t time.Time) error {
	logrus.Printf("设置系统时间为: %s", t.Format("2006-01-02 15:04:05.000 UTC"))

	switch runtime.GOOS {
	case "linux":
		return tc.setLinuxTime(t)
	case "windows":
		return tc.setWindowsTime(t)
	case "darwin":
		return tc.setMacTime(t)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// setLinuxTime 设置Linux系统时间
func (tc *TimeClient) setLinuxTime(t time.Time) error {
	// 方法1: 使用timedatectl (systemd)
	isoTime := t.UTC().Format("2006-01-02 15:04:05")
	cmd := exec.Command("timedatectl", "set-time", isoTime)
	if err := cmd.Run(); err != nil {
		// 方法2: 使用date命令
		dateStr := t.UTC().Format("010215042006.05") // MMDDhhmmYYYY.ss
		cmd = exec.Command("date", "-u", dateStr)
		if err2 := cmd.Run(); err2 != nil {
			return fmt.Errorf("failed to set linux time: timedatectl=%v, date=%v", err, err2)
		}
	}

	logrus.Printf("Linux系统时间设置成功")
	return nil
}

// setWindowsTime 设置Windows系统时间
func (tc *TimeClient) setWindowsTime(t time.Time) error {
	// 使用w32tm进行时间同步
	cmd := exec.Command("w32tm", "/resync", "/force")
	if err := cmd.Run(); err != nil {
		// 备用方法: 使用PowerShell设置时间
		timeStr := t.Format("01/02/2006 15:04:05")
		psCmd := fmt.Sprintf("Set-Date -Date '%s'", timeStr)
		cmd = exec.Command("powershell", "-Command", psCmd)
		if err2 := cmd.Run(); err2 != nil {
			return fmt.Errorf("failed to set windows time: w32tm=%v, powershell=%v", err, err2)
		}
	}

	logrus.Printf("Windows系统时间设置成功")
	return nil
}

// setMacTime 设置macOS系统时间
func (tc *TimeClient) setMacTime(t time.Time) error {
	// 方法1: 使用date命令
	dateStr := t.UTC().Format("0102150406") // MMDDhhmmyy
	cmd := exec.Command("date", "-u", dateStr)
	if err := cmd.Run(); err != nil {
		// 方法2: 使用sntp
		cmd = exec.Command("sntp", "-sS", tc.config.ServerURL)
		if err2 := cmd.Run(); err2 != nil {
			return fmt.Errorf("failed to set macOS time: date=%v, sntp=%v", err, err2)
		}
	}

	logrus.Printf("macOS系统时间设置成功")
	return nil
}

// setSystemTimeAlternative 备用的系统时间设置方法
func (tc *TimeClient) setSystemTimeAlternative(t time.Time) error {
	logrus.Printf("尝试备用时间设置方法...")

	switch runtime.GOOS {
	case "linux":
		// 尝试使用ntpdate
		cmd := exec.Command("ntpdate", "-s", "-u", tc.extractHostFromURL(tc.config.ServerURL))
		if err := cmd.Run(); err != nil {
			// 尝试使用chrony
			cmd = exec.Command("chrony", "sources", "-v")
			if err2 := cmd.Run(); err2 != nil {
				return fmt.Errorf("linux fallback methods failed: ntpdate=%v, chrony=%v", err, err2)
			}
		}
	case "windows":
		// 尝试使用net time
		cmd := exec.Command("net", "time", "/set", "/y")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("windows fallback method failed: %v", err)
		}
	case "darwin":
		// 尝试使用系统偏好设置命令
		cmd := exec.Command("systemsetup", "-setnetworktimeserver", tc.extractHostFromURL(tc.config.ServerURL))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("macOS fallback method failed: %v", err)
		}
	default:
		return fmt.Errorf("unsupported OS for fallback methods: %s", runtime.GOOS)
	}

	logrus.Printf("备用方法执行成功")
	return nil
}

// extractHostFromURL 从URL中提取主机名
func (tc *TimeClient) extractHostFromURL(url string) string {
	// 简单的URL解析，提取主机名
	if len(url) > 7 && url[:7] == "http://" {
		url = url[7:]
	} else if len(url) > 8 && url[:8] == "https://" {
		url = url[8:]
	}

	// 移除路径部分
	if idx := len(url); idx > 0 {
		for i, r := range url {
			if r == '/' || r == ':' {
				idx = i
				break
			}
		}
		return url[:idx]
	}
	return url
}

// verifyTimeAdjustment 验证时间调整结果
func (tc *TimeClient) verifyTimeAdjustment(targetTime time.Time) {
	logrus.Printf("验证时间调整结果...")

	// 等待系统时间稳定
	time.Sleep(2 * time.Second)

	currentTime := time.Now().UTC()
	timeDiff := currentTime.Sub(targetTime)
	if timeDiff < 0 {
		timeDiff = -timeDiff
	}

	logrus.Printf("   目标时间: %s", targetTime.Format("2006-01-02 15:04:05.000 UTC"))
	logrus.Printf("   当前时间: %s", currentTime.Format("2006-01-02 15:04:05.000 UTC"))
	logrus.Printf("   时间差异: %v", timeDiff)

	if timeDiff < 2*time.Second {
		logrus.Printf("时间调整验证成功，差异在可接受范围内")
	} else if timeDiff < 10*time.Second {
		logrus.Printf("时间调整基本成功，但仍有 %v 的差异", timeDiff)
	} else {
		logrus.Printf("时间调整可能失败，差异过大: %v", timeDiff)
		logrus.Printf("建议: 请检查系统时间服务配置或手动同步")
	}
}

// defaultConfig 默认配置
func DefaultConfig() *ClientConfig {
	return &ClientConfig{
		ServerURL:    "http://time-server:8080",
		ClientID:     "client-001",
		Region:       "asia-east1",
		Timezone:     "Asia/Shanghai",
		SyncInterval: 1 * time.Hour,
		Timeout:      10 * time.Second,
		MaxRetries:   3,
		EnableTLS:    false,
	}
}

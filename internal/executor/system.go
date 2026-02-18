package executor

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// SystemMetrics gathers CPU, memory, disk, and uptime metrics (exported for use by main ticker).
func SystemMetrics() (map[string]interface{}, error) {
	data := map[string]interface{}{
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
	}

	// CPU usage (1-second sample)
	cpuPercent, err := cpu.Percent(1*time.Second, false)
	if err == nil && len(cpuPercent) > 0 {
		data["cpu_percent"] = cpuPercent[0]
	}

	// Memory
	vmem, err := mem.VirtualMemory()
	if err == nil {
		data["memory_total_bytes"] = vmem.Total
		data["memory_used_bytes"] = vmem.Used
		data["memory_percent"] = vmem.UsedPercent
	}

	// Disk (root)
	diskUsage, err := disk.Usage("/")
	if err == nil {
		data["disk_total_bytes"] = diskUsage.Total
		data["disk_used_bytes"] = diskUsage.Used
		data["disk_percent"] = diskUsage.UsedPercent
	}

	// Uptime
	uptime, err := host.Uptime()
	if err == nil {
		data["uptime_seconds"] = uptime
	}

	return data, nil
}

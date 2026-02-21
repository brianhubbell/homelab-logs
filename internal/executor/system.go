package executor

import (
	"context"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"github.com/shirou/gopsutil/v4/sensors"
)

// SystemMetrics gathers all available system metrics via gopsutil.
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

	// CPU counts
	if logical, err := cpu.Counts(true); err == nil {
		data["cpu_count_logical"] = logical
	}
	if physical, err := cpu.Counts(false); err == nil {
		data["cpu_count_physical"] = physical
	}

	// Memory
	vmem, err := mem.VirtualMemory()
	if err == nil {
		data["memory_total_bytes"] = vmem.Total
		data["memory_used_bytes"] = vmem.Used
		data["memory_percent"] = vmem.UsedPercent
	}

	// Swap
	swap, err := mem.SwapMemory()
	if err == nil {
		data["swap_total_bytes"] = swap.Total
		data["swap_used_bytes"] = swap.Used
		data["swap_percent"] = swap.UsedPercent
	}

	// Disk (root)
	diskUsage, err := disk.Usage("/")
	if err == nil {
		data["disk_total_bytes"] = diskUsage.Total
		data["disk_used_bytes"] = diskUsage.Used
		data["disk_percent"] = diskUsage.UsedPercent
	}

	// Disk I/O
	diskIO, err := disk.IOCounters()
	if err == nil {
		disks := make(map[string]map[string]interface{}, len(diskIO))
		for name, d := range diskIO {
			disks[name] = map[string]interface{}{
				"read_count":  d.ReadCount,
				"write_count": d.WriteCount,
				"read_bytes":  d.ReadBytes,
				"write_bytes": d.WriteBytes,
				"read_time":   d.ReadTime,
				"write_time":  d.WriteTime,
			}
		}
		data["disk_io"] = disks
	}

	// Uptime
	uptime, err := host.Uptime()
	if err == nil {
		data["uptime_seconds"] = uptime
	}

	// Boot time
	bootTime, err := host.BootTime()
	if err == nil {
		data["boot_time"] = bootTime
	}

	// Load averages
	loadAvg, err := load.Avg()
	if err == nil {
		data["load_avg_1"] = loadAvg.Load1
		data["load_avg_5"] = loadAvg.Load5
		data["load_avg_15"] = loadAvg.Load15
	}

	// Network I/O per interface
	netIO, err := net.IOCounters(true)
	if err == nil {
		interfaces := make([]map[string]interface{}, 0, len(netIO))
		for _, iface := range netIO {
			interfaces = append(interfaces, map[string]interface{}{
				"name":         iface.Name,
				"bytes_sent":   iface.BytesSent,
				"bytes_recv":   iface.BytesRecv,
				"packets_sent": iface.PacketsSent,
				"packets_recv": iface.PacketsRecv,
				"errin":        iface.Errin,
				"errout":       iface.Errout,
				"dropin":       iface.Dropin,
				"dropout":      iface.Dropout,
			})
		}
		data["network_interfaces"] = interfaces
	}

	// Process count
	pids, err := process.Pids()
	if err == nil {
		data["process_count"] = len(pids)
	}

	// Temperatures (best-effort)
	temps, err := sensors.TemperaturesWithContext(context.Background())
	if err == nil && len(temps) > 0 {
		tempList := make([]map[string]interface{}, 0, len(temps))
		for _, t := range temps {
			tempList = append(tempList, map[string]interface{}{
				"sensor":      t.SensorKey,
				"temperature": t.Temperature,
			})
		}
		data["temperatures"] = tempList
	}

	return data, nil
}

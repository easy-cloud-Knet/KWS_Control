package startup

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/easy-cloud-Knet/KWS_Control/structure"
	"github.com/easy-cloud-Knet/KWS_Control/util"
	"gopkg.in/yaml.v3"
)

func readConfig(path string) (structure.Config, error) {
	log := util.GetLogger()

	// 받아온 path 읽기
	config, err := tryReadConfig(path)
	if err == nil {
		log.DebugInfo("Successfully read config from: %s", path)
		return config, nil
	}

	log.Warn("Failed to read config from %s: %v", path, err)

	fallbackPath := filepath.Join("resources", "config.yaml")
	log.Info("Attempting to read config from fallback path: %s", fallbackPath)

	// 받아온 path에 없으면 resources/에 있는 config.yaml 가져오기
	config, fallbackErr := tryReadConfig(fallbackPath)
	if fallbackErr != nil {
		return structure.Config{}, fmt.Errorf("both original path (%s) and fallback path (%s) failed. Original error: %w, Fallback error: %v", path, fallbackPath, err, fallbackErr)
	}

	log.Info("Successfully read config from fallback path: %s", fallbackPath)
	return config, nil
}

// tryReadConfig attempts to read and parse a config file from the given path
func tryReadConfig(path string) (structure.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return structure.Config{}, fmt.Errorf("failed to open config file: %w", err)
	}

	//goland:noinspection GoUnhandledErrorResult
	defer file.Close()

	var config structure.Config
	data, err := os.ReadFile(path)
	if err != nil {
		return structure.Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return structure.Config{}, fmt.Errorf("failed to decode config file: %w", err)
	}

	applyAllocDefaults(&config)

	return config, nil
}

// applyAllocDefaults는 코어 선택 파라미터에 env 오버라이드를 적용한 뒤 안전한 기본값으로 보정한다.
// env 패턴은 init.go의 DB 설정과 동일(env 우선, 없으면 config 값 사용).
//   - cpu_overcommit ≤ 0  → 1.0 (오버커밋 없음)
//   - *_reserve_pct  범위 밖([0,1)) → 0.0 (여유분 없음)
func applyAllocDefaults(config *structure.Config) {
	log := util.GetLogger()

	if v := os.Getenv("CPU_OVERCOMMIT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			config.CpuOvercommit = f
		} else {
			log.Warn("invalid CPU_OVERCOMMIT=%q, ignoring: %v", v, err)
		}
	}
	if v := os.Getenv("MEM_RESERVE_PCT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			config.MemReservePct = f
		} else {
			log.Warn("invalid MEM_RESERVE_PCT=%q, ignoring: %v", v, err)
		}
	}
	if v := os.Getenv("DISK_RESERVE_PCT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			config.DiskReservePct = f
		} else {
			log.Warn("invalid DISK_RESERVE_PCT=%q, ignoring: %v", v, err)
		}
	}

	if config.CpuOvercommit <= 0 {
		config.CpuOvercommit = 1.0
	}
	if config.MemReservePct < 0 || config.MemReservePct >= 1 {
		config.MemReservePct = 0.0
	}
	if config.DiskReservePct < 0 || config.DiskReservePct >= 1 {
		config.DiskReservePct = 0.0
	}

	log.DebugInfo("alloc params: cpu_overcommit=%.2f, mem_reserve_pct=%.2f, disk_reserve_pct=%.2f",
		config.CpuOvercommit, config.MemReservePct, config.DiskReservePct)
}

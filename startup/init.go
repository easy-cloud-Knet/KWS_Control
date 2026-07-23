package startup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/easy-cloud-Knet/KWS_Control/client"
	"github.com/easy-cloud-Knet/KWS_Control/structure"
	"golang.org/x/sync/errgroup"

	_ "github.com/go-sql-driver/mysql"
	_ "gopkg.in/yaml.v3"

	"github.com/easy-cloud-Knet/KWS_Control/util"
)

func InitializeCoreData(configPath string) (structure.ControlContext, error) {
	log := util.GetLogger()

	var infra structure.ControlContext

	config, err := readConfig(configPath)
	if err != nil {
		return structure.ControlContext{}, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// ---------------- Main DB connection -----------------------
	dbUserMain := os.Getenv("DB_USER")
	if dbUserMain == "" {
		dbUserMain = config.DB.User
		if dbUserMain == "" {
			dbUserMain = "root"
		}
	}

	dbPasswordMain := os.Getenv("DB_PASSWORD")
	if dbPasswordMain == "" {
		dbPasswordMain = config.DB.Password
	}

	dbHostMain := os.Getenv("DB_HOST")
	if dbHostMain == "" {
		dbHostMain = config.DB.Host
		if dbHostMain == "" {
			dbHostMain = "localhost"
		}
	}

	dbPortMain := os.Getenv("DB_PORT")
	if dbPortMain == "" {
		if config.DB.Port != 0 {
			dbPortMain = strconv.Itoa(config.DB.Port)
		} else {
			dbPortMain = "3306"
		}
	}

	dbNameMain := os.Getenv("DB_NAME")
	if dbNameMain == "" {
		dbNameMain = config.DB.Name
		if dbNameMain == "" {
			dbNameMain = "db"
		}
	}

	dsnMain := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUserMain, dbPasswordMain, dbHostMain, dbPortMain, dbNameMain)

	log.Info("database connection info: %s:%s@tcp(%s:%s)/%s", dbUserMain, dbPasswordMain, dbHostMain, dbPortMain, dbNameMain)
	mainDB, err := sql.Open("mysql", dsnMain)

	log.Info("generic database connection opened", true)
	log.Info("generic database stats: %v", mainDB.Stats(), true)

	if err != nil {
		log.DebugError("failed to open generic database connection: %v", err)
		return structure.ControlContext{}, fmt.Errorf("failed to open generic database connection: %w", err)
	}
	if err := mainDB.Ping(); err != nil {
		log.DebugError("failed to ping generic database: %v", err)
		return structure.ControlContext{}, fmt.Errorf("failed to ping generic database: %w", err)
	}

	// ---------------- Guacamole DB connection --------------------

	dbUser := os.Getenv("GUAC_DB_USER")
	if dbUser == "" {
		dbUser = config.GuacDB.User
		if dbUser == "" {
			dbUser = "root"
		}
	}

	dbPassword := os.Getenv("GUAC_DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = config.GuacDB.Password
	}

	dbHost := os.Getenv("GUAC_DB_HOST")
	if dbHost == "" {
		dbHost = config.GuacDB.Host
		if dbHost == "" {
			dbHost = "localhost"
		}
	}

	dbPort := os.Getenv("GUAC_DB_PORT")
	if dbPort == "" {
		if config.GuacDB.Port != 0 {
			dbPort = strconv.Itoa(config.GuacDB.Port)
		} else {
			dbPort = "3306"
		}
	}

	dbName := os.Getenv("GUAC_DB_NAME")
	if dbName == "" {
		dbName = config.GuacDB.Name
		if dbName == "" {
			dbName = "guacamole_db"
		}
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return structure.ControlContext{}, fmt.Errorf("failed to open database connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		return structure.ControlContext{}, fmt.Errorf("failed to ping database: %w", err)
	}

	guacBaseURL := os.Getenv("GUACAMOLE_BASE_URL")
	log.Println("GUACAMOLE_BASE_URL:", guacBaseURL)
	if guacBaseURL != "" {
		config.GuacBaseURL = guacBaseURL
	}
	if config.GuacBaseURL == "" {
		config.GuacBaseURL = "http://100.101.247.128:8080/guacamole"
	}

	infra.Config = config
	infra.GuacDB = db
	infra.DB = mainDB
	infra.VMRepo = structure.NewMySQLVMRepository(mainDB)
	infra.Resources = structure.NewResourceManager()

	var last_subnet_db string
	err = infra.DB.QueryRow("SELECT last_subnet from core_base.subnet where id = '1'").Scan(&last_subnet_db)
	if err != nil {
		return structure.ControlContext{}, fmt.Errorf("failed to get last_subnet: %w", err)
	}
	infra.Last_subnet = last_subnet_db
	// 모든 Core 정의
	for i := range infra.Resources.Cores {
		for vmUUID := range infra.Resources.Cores[i].VMInfoIdx {
			infra.Resources.VMLocation[vmUUID] = &infra.Resources.Cores[i]
		}
		infra.Resources.Cores[i].IsAlive = false
	}

	// config에 설정된 코어에 대해서 정보 업데이트

	// 환경변수에서 먼저 설정을 불러오고 값이 없을 때만 config 파일에서 가져옴
	var coreAddresses []string
	coresEnv := os.Getenv("CORES")
	if coresEnv != "" {
		coreAddresses = strings.Split(coresEnv, ",")
	} else {
		coreAddresses = config.Cores
	}

	g, ctx := errgroup.WithContext(context.Background())
	for _, coreAddress := range coreAddresses {
		parts := strings.Split(coreAddress, ":")
		if len(parts) != 2 {
			return structure.ControlContext{}, fmt.Errorf("invalid core address format in '%s': expected ip:port", coreAddress)
		}
		ip := parts[0]
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return structure.ControlContext{}, fmt.Errorf("error converting port number from %s: %w", coreAddress, err)
		}

		core := findCore(infra.Resources.Cores, ip, uint16(port))
		if core == nil {
			newCore := structure.Core{
				IP:      ip,
				Port:    uint16(port),
				IsAlive: true,
			}
			infra.Resources.Cores = append(infra.Resources.Cores, newCore)
			core = &infra.Resources.Cores[len(infra.Resources.Cores)-1]
			log.DebugInfo("Added new core: %s:%d", ip, port)
		} else {
			core.IsAlive = true
		}

		currentCore := core
		g.Go(func() error {
			coreClient := client.NewCoreClient(currentCore)

			memResp, err := coreClient.GetCoreMachineMemoryInfo(ctx)
			if err != nil {
				currentCore.IsAlive = false
				return fmt.Errorf("failed to get Memory info for core %s:%d: %w", currentCore.IP, currentCore.Port, err)
			}
			diskResp, err := coreClient.GetCoreMachineDiskInfo(ctx)
			if err != nil {
				currentCore.IsAlive = false
				return fmt.Errorf("failed to get Disk info for core %s:%d: %w", currentCore.IP, currentCore.Port, err)
			}
			cpuResp, err := coreClient.GetCoreMachineCpuInfo(ctx)
			if err != nil {
				currentCore.IsAlive = false
				return fmt.Errorf("failed to get CPU info for core %s:%d: %w", currentCore.IP, currentCore.Port, err)
			}
			if cpuResp == nil || cpuResp.VcpuStatus == nil {
				currentCore.IsAlive = false
				return fmt.Errorf("core %s:%d returned no vcpu_status in CPU info", currentCore.IP, currentCore.Port)
			}

			totalMemoryMiB := uint32(memResp.Total * 1024)
			totalDiskMiB := uint32(diskResp.Total * 1024)
			freeMemoryMiB := uint32(memResp.Available * 1024)
			freeDiskMiB := uint32(diskResp.Free * 1024)

			totalCpuCores := cpuResp.VcpuStatus.Total
			freeCpuCores := cpuResp.VcpuStatus.Idle

			currentCore.CoreInfoIdx.Cpu = totalCpuCores
			currentCore.CoreInfoIdx.Memory = totalMemoryMiB
			currentCore.CoreInfoIdx.Disk = totalDiskMiB

			currentCore.FreeDisk = freeDiskMiB
			currentCore.FreeMemory = freeMemoryMiB
			currentCore.FreeCPU = freeCpuCores

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return structure.ControlContext{}, fmt.Errorf("failed to get core info: %w", err)
	}

	vmInfoList, coreIdxList, err := infra.VMRepo.GetAllInstanceInfo()
	if err != nil {
		return structure.ControlContext{}, fmt.Errorf("failed to get all instance info: %w", err)
	}

	for i, vmInfo := range vmInfoList {
		coreIdx := coreIdxList[i]
		if coreIdx < 0 || coreIdx >= len(infra.Resources.Cores) {
			return structure.ControlContext{}, fmt.Errorf("core index %d out of range for core list", coreIdx)
		}
		core := &infra.Resources.Cores[coreIdx]
		if core.VMInfoIdx == nil {
			core.VMInfoIdx = make(map[structure.UUID]*structure.VMInfo)
		}
		if _, exists := core.VMInfoIdx[vmInfo.UUID]; !exists {
			core.VMInfoIdx[vmInfo.UUID] = &vmInfo
			infra.Resources.VMLocation[vmInfo.UUID] = core
		} else {
			log.DebugInfo("VM %s already exists in core %d, skipping", vmInfo.UUID, coreIdx)
		}
	}

	return infra, nil
}

func findCore(cores []structure.Core, ip string, port uint16) *structure.Core {
	for i := range cores {
		if cores[i].IP == ip && cores[i].Port == port {
			return &cores[i]
		}
	}
	return nil
}

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/easy-cloud-Knet/KWS_Control/structure"

	"github.com/easy-cloud-Knet/KWS_Control/api"
	"github.com/easy-cloud-Knet/KWS_Control/service"
	"github.com/easy-cloud-Knet/KWS_Control/startup"
	"github.com/easy-cloud-Knet/KWS_Control/util"
)

func main() {
	log := util.GetLogger()

	ctx := context.Background()

	//Redis 초기화 (VM status 저장 + 코어별 할당 집계 공용)
	rdb, err := startup.InitializeRedis(ctx)
	if err != nil {
		log.Error("Failed to initialize Redis: %v", err, true)
		panic(err)
	}

	log.Info("KWS Control Server Starting...", true)

	contextStruct, err := startup.InitializeCoreData("config.yaml")
	if err != nil {
		log.Error("Failed to initialize: %v", err, true)
		panic(err)
	}
	printCores(contextStruct.Cores)

	// DB 인스턴스 합계로 코어별 할당(core:{ip}:{port}:alloc) 재구성(시작 시 1회, 멱등). 실패해도 기동은 계속.
	if err := service.RebuildCoreAllocFromDB(ctx, &contextStruct, rdb); err != nil {
		log.Error("Failed to rebuild core alloc from DB: %v", err, true)
	}

	// 주기적 헬스체크(코어 가용성/용량 갱신)
	go service.StartHealthcheck(ctx, &contextStruct, 30*time.Second)

	go func() {
		err := api.Server(contextStruct.Config.Port, &contextStruct, rdb)
		if err != nil {
			log.Error("Failed to start server: %v", err, true)
			panic(err)
		}
	}()
	select {}
	//fuck
}

func printCores(cores []structure.Core) {
	for i, core := range cores {
		fmt.Printf("Core #%d: %s\n", i, core.IP)
		fmt.Printf("  * IsAlive: %t\n", core.IsAlive)
		fmt.Printf("  * FreeMemory(GiB): %.0f\n", float64(core.FreeMemory)/1024)
		fmt.Printf("  * FreeCPU: %d\n", core.FreeCPU)
		fmt.Printf("  * FreeDisk(GiB): %.0f\n", float64(core.FreeDisk)/1024)
	}
}

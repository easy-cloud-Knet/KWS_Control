package vms

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-ping/ping"
)


var Computers []Computer

func HeartBeatSensor() {
    for {
        var wg sync.WaitGroup
        for i := range Computers {
            wg.Add(1)
            go func(c *Computer) {
                defer wg.Done()
                checkComputer(c)
            }(&Computers[i])
        }
        wg.Wait()
        time.Sleep(time.Second * 10)
    }
}

func checkComputer(c *Computer) {
    currState := c.CheckRunning()
    if c.IsAlive != currState {
        if !currState {
            fmt.Printf("Computer %s is down. Need to switch to other host\n", c.Name)
        } else {
            fmt.Printf("Computer %s is now on. Analyzing why it was turned on\n", c.Name)
        }
        c.IsAlive = currState
    }

    if c.IsAlive {
        var vmWg sync.WaitGroup
        for i := range c.Allocated {
            vmWg.Add(1)
            go func(v *VM) {
                defer vmWg.Done()
                checkVM(v)
            }(&c.Allocated[i])
        }
        vmWg.Wait()
    }
}

func checkVM(v *VM) {
    currState := v.CheckRunning()
    if v.IsAlive != currState {
        if !currState {
            fmt.Printf("VM %s is down. Need to reallocate\n", v.Name)
        } else {
            fmt.Printf("VM %s is now on. Analyzing startup reason\n", v.Name)
        }
        v.IsAlive = currState
    }
}

func (c *Computer) CheckRunning() bool {
    isAlive := PingCheck(c.IP)
    fmt.Printf("Computer %s is %s\n", c.Name, statusString(isAlive))
    return isAlive
}

func (v *VM) CheckRunning() bool {
    isAlive := PingCheck(v.IP)
    fmt.Printf("VM %s is %s\n", v.Name, statusString(isAlive))
    return isAlive
}

func PingCheck(ip string) bool {
    pinger, err := ping.NewPinger(ip)
    if err != nil {
        fmt.Printf("Error creating pinger for %s: %v\n", ip, err)
        return false
    }
    pinger.Count = 3
    pinger.Timeout = time.Second * 2
    err = pinger.Run()
    if err != nil {
        fmt.Printf("Error pinging %s: %v\n", ip, err)
        return false
    }
    return pinger.Statistics().PacketsRecv > 0
}

func statusString(isAlive bool) string {
    if isAlive {
        return "on"
    }
    return "down"
}

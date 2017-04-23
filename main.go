package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
    "path/filepath"
    "os/user"
    "strings"
    flags "github.com/jessevdk/go-flags"
)

const (
    Author  = "webdevops.io"
    Version = "0.1.0"
)

var opts struct {
    Processes                 int       `           long:"processes"            description:"Number of parallel executions" default:"1"`
    DefaultUser               string    `           long:"default-user"         description:"Default user"                  default:"root"`
    IncludeCronD              []string  `           long:"include"              description:"Include files in directory as system crontabs (with user)"`
    RunParts                  []string  `           long:"run-parts"            description:"Include files in directory with dynamic time execution (time-spec:path)"`
    RunParts1m                []string  `           long:"run-parts-1min"       description:"Include files in directory every minute execution (run-part)"`
    RunPartsHourly            []string  `           long:"run-parts-hourly"     description:"Include files in directory every hour execution (run-part)"`
    RunPartsDaily             []string  `           long:"run-parts-daily"      description:"Include files in directory every day execution (run-part)"`
    RunPartsWeekly            []string  `           long:"run-parts-weekly"     description:"Include files in directory every week execution (run-part)"`
    RunPartsMonthly           []string  `           long:"run-parts-monthly"    description:"Include files in directory every month execution (run-part)"`
    ShowVersion               bool      `short:"V"  long:"version"              description:"show version and exit"`
    ShowHelp                  bool      `short:"h"  long:"help"                 description:"show this help message"`
}

var argparser *flags.Parser
var args []string
func initArgParser() ([]string) {
    var err error
    argparser = flags.NewParser(&opts, flags.PassDoubleDash)
    args, err = argparser.Parse()

    // check if there is an parse error
    if err != nil {
        logFatalErrorAndExit(err, 1)
    }

    // --version
    if (opts.ShowVersion) {
        fmt.Println(fmt.Sprintf("go-crond version %s", Version))
        fmt.Println(fmt.Sprintf("Copyright (C) 2017 %s", Author))
        os.Exit(0)
    }

    // --help
    if (opts.ShowHelp) {
        argparser.WriteHelp(os.Stdout)
        os.Exit(1)
    }

    return args
}

var LoggerInfo *log.Logger
var LoggerError *log.Logger
func initLogger() {
    LoggerInfo = log.New(os.Stdout, "go-crond: ", 0)
    LoggerError = log.New(os.Stderr, "go-crond: ", 0)
}

// Log error object as message
func logFatalErrorAndExit(err error, exitCode int) {
    LoggerError.Fatalf("ERROR: %s\n", err)
    os.Exit(exitCode)
}

func checkIfFileIsValid(f os.FileInfo, path string) bool {
    if f.Mode().IsRegular() {
        if f.Mode().Perm() & 0022 == 0 {
            return true
        } else {
            LoggerInfo.Printf("Ignoring file with wrong modes (not xx22) %s\n", path)
        }
    } else {
        LoggerInfo.Printf("Ignoring non regular file %s\n", path)
    }

    return false
}

func findFilesInPaths(pathlist []string, callback func(os.FileInfo, string)) {
    for i := range pathlist {
        filepath.Walk(pathlist[i], func(path string, f os.FileInfo, err error) error {
            path, _ = filepath.Abs(path)

            if f.IsDir() {
                return nil
            }

            if checkIfFileIsValid(f, path) {
                callback(f, path)
            }

            return nil
        })
    }
}

func findExecutabesInPathes(pathlist []string, callback func(os.FileInfo, string)) {
    findFilesInPaths(pathlist, func(f os.FileInfo, path string) {
        if f.Mode().IsRegular() && (f.Mode().Perm() & 0100 != 0) {
            callback(f, path)
        } else {
            LoggerInfo.Printf("Ignoring non exectuable file %s\n", path)
        }
    })
}

func parseCrontab(path string) []CrontabEntry {
	file, err := os.Open(path)
	if err != nil {
		LoggerError.Fatalf("crontab path: %v err:%v", path, err)
	}

	parser, err := NewParser(file)
	if err != nil {
		LoggerError.Fatalf("Parser read err: %v", err)
	}

    crontabEntries := parser.Parse()

    return crontabEntries
}

func collectCrontabs(args []string) []CrontabEntry {
    var ret []CrontabEntry

    // args: crontab files as normal arguments
    for i := range args {
        crontabFile, err := filepath.Abs(args[i])
        if err != nil {
            LoggerError.Fatalf("Invalid file: %v", err)
        }

        f, err := os.Lstat(crontabFile)
        if err != nil {
            LoggerError.Fatalf("File stats failed: %v", err)
        }
        if checkIfFileIsValid(f, crontabFile) {
            entries := parseCrontab(crontabFile)

            ret = append(ret, entries...)
        }
    }

    // --include-crond
    if len(opts.IncludeCronD) >= 1 {
        findFilesInPaths(opts.IncludeCronD, func(f os.FileInfo, path string) {
            entries := parseCrontab(path)
            ret = append(ret, entries...)
        })
    }

    // --run-parts
    if len(opts.RunParts) >= 1 {
        for i := range opts.RunParts {
            runPart := opts.RunParts[i]

            if strings.Contains(runPart, ":") {
                split := strings.SplitN(runPart, ":", 2)
                cronSpec, cronPath := split[0], split[1]

                var cronPaths []string
                cronPaths = append(cronPaths, cronPath)

                findExecutabesInPathes(cronPaths, func(f os.FileInfo, path string) {
                    ret = append(ret, CrontabEntry{"@every " + cronSpec, opts.DefaultUser, path})
                })
            } else {
                LoggerError.Printf("Ignoring --run-parts because of missing time spec: %s\n", runPart)
            }
        }
    }

    // --run-parts-minute
    if len(opts.RunParts1m) >= 1 {
        findExecutabesInPathes(opts.RunParts1m, func(f os.FileInfo, path string) {
            ret = append(ret, CrontabEntry{"@every 1m", opts.DefaultUser, path})
        })
    }

    // --run-parts-hourly
    if len(opts.RunPartsHourly) >= 1 {
        findExecutabesInPathes(opts.RunPartsHourly, func(f os.FileInfo, path string) {
            ret = append(ret, CrontabEntry{"@hourly", opts.DefaultUser, path})
        })
    }

    // --run-parts-daily
    if len(opts.RunPartsDaily) >= 1 {
        findExecutabesInPathes(opts.RunPartsDaily, func(f os.FileInfo, path string) {
            ret = append(ret, CrontabEntry{"@daily", opts.DefaultUser, path})
        })
    }

    // --run-parts-weekly
    if len(opts.RunPartsWeekly) >= 1 {
        findExecutabesInPathes(opts.RunPartsWeekly, func(f os.FileInfo, path string) {
            ret = append(ret, CrontabEntry{"@weekly", opts.DefaultUser, path})
        })
    }

    // --run-parts-monthly
    if len(opts.RunPartsMonthly) >= 1 {
        findExecutabesInPathes(opts.RunPartsMonthly, func(f os.FileInfo, path string) {
            ret = append(ret, CrontabEntry{"@monthly", opts.DefaultUser, path})
        })
    }

    return ret
}

func main() {
    initLogger()
    args := initArgParser()

    LoggerInfo.Printf("Starting version %s", Version)

    var wg sync.WaitGroup

    enableUserSwitch := true

    currentUser, _ := user.Current()
    if currentUser.Uid != "0" {
        LoggerError.Println("WARNING: go-crond is NOT running as root, disabling user switching")
        enableUserSwitch = false
    }

    crontabEntries := collectCrontabs(args)

	runtime.GOMAXPROCS(opts.Processes)
    runner := NewRunner()

    for i := range crontabEntries {
        crontabEntry := crontabEntries[i]

        if enableUserSwitch {
            runner.AddWithUser(crontabEntry.Spec, crontabEntry.User, crontabEntry.Command)
        } else {
            runner.Add(crontabEntry.Spec, crontabEntry.Command)
        }
    }

    registerRunnerShutdown(runner, &wg)
    runner.Start()
    wg.Add(1)
	wg.Wait()

	LoggerInfo.Println("Terminated")
}

func registerRunnerShutdown(runner *Runner, wg *sync.WaitGroup) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-c
		LoggerInfo.Println("Got signal: ", s)
		runner.Stop()
		wg.Done()
	}()
}

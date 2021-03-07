package utils

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// global variable
var Profiler *bufio.Writer

func InitFunctionProfiler(fname string) {
	// extract path from given file name
	path, err := os.Getwd()
	if err != nil {
		log.Printf("ERROR: fail to read from %v, error %v\n", path, err)
		return
	}
	if fname == "" {
		fname = "das-profile.log"
	} else {
		arr := strings.Split(fname, "/")
		path = strings.Join(arr[:len(arr)-1], "/")
		fname = arr[len(arr)-1]
	}
	// create the log directory
	if err := os.MkdirAll(path, 0755); err != nil {
		log.Printf("ERROR: fail to make %s, error %v\n", path, err)
		return
	}
	// open the log file
	fname = fmt.Sprintf("%s/%s", path, fname)
	file, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("ERROR: fail to open %s, error %v\n", fname, err)
		return
	}
	defer file.Close()
	Profiler = bufio.NewWriter(file)
	if WEBSERVER != 0 {
		log.Println("DAS profiler", fname)
	}
}

// Latency Measurement of individual component of the codebase
// https://medium.com/swlh/easy-guide-to-latency-measurement-in-golang-38c3297ebbd2
// Usage, put the following statement in any function we need to measure:
// defer measureTime("funcName")
func MeasureTime(funcName string) func() {
	start := time.Now()
	return func() {
		if Profiler != nil {
			fmt.Fprintf(Profiler, "%s %s %v \n", start.Format("20060102150405"), funcName, time.Since(start))
			Profiler.Flush()
		}
	}
}

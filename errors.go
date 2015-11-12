package errors

//http://www.goinggo.net/2013/11/using-log-package-in-go.html
import (
	"bytes"
	"errors"
	"fmt"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	//github.com/cloudfoundry/gosigar
	"io"
	"log"
	"os"
	"runtime"
	"time"
)

const (
	E = "- GOT:"
	M = "MSG:"
	S = "|"
	V = "- VAL:"

	Expires      = 60 * 60 * 24 * 10
	IntervalPoll = 60 * time.Second
	PollTime     = 20 * time.Minute
	PollCapacity = int(PollTime / IntervalPoll)

	BYTE     = 1.0
	KILOBYTE = 1024 * BYTE
	MEGABYTE = 1024 * KILOBYTE
	GIGABYTE = 1024 * MEGABYTE

	FileLog   = "data/log.txt"
	FileStats = "data/stats.txt"
)

var (
	StatsLog *log.Logger
	InfoLog  *log.Logger
	WarnLog  *log.Logger
	ErrLog   *log.Logger
	CritLog  *log.Logger
	FatalLog *log.Logger

	DevEnv = false
)

type (
	Percent uint8

	stats struct {
		MemUsed uint64
		Memfree Percent
		CPUUsed Percent
	}
	statCollector struct {
		w io.Writer // Underlying writer to send data to
		b bytes.Buffer
	}
	Counter struct {
		w io.Writer // Underlying writer to send data to
		c int       // Number of bytes written since last call to Count()
	}
)

func init() {
	os.Mkdir("data", 0777)
	println("Error & Logging init done")
}

func (q *statCollector) Write(p []byte) (n int, err error) {
	q.b.Write(p)
	return 0, nil
}
func (q *statCollector) Flush() error {
	q.w.Write(q.b.Bytes())
	return nil
}

func NewLogCounter(w io.Writer) (h *Counter) {
	h = new(Counter)
	h.w = w
	return
}

// Write: standard io.Writer interface. To use this package call
// Write continually. This will both count the bytes written and
// write to the underlying writer.
func (h *Counter) Write(p []byte) (n int, err error) {
	n, err = h.w.Write(p)
	h.c += n
	return
}

// Count: returns the total number of bytes that were written to the
// underlying writer since the last call to Count
func (h *Counter) Count() (c int) {
	c = h.c
	h.c = 0
	return
}

func logPoll(n time.Duration, done chan bool) {
	flag := os.O_CREATE | os.O_WRONLY | os.O_APPEND

	fileLog, err := os.OpenFile(FileLog, flag, 0666)
	if err != nil {
		FatalLog.Fatalln("Failed to open log file:", err)
	}
	fileStats, err := os.OpenFile(FileStats, flag, 0666)
	if err != nil {
		FatalLog.Fatalln("Failed to open stats file:", err)
	}
	outLog := io.MultiWriter(fileLog, os.Stdout)

	StatCollect := &statCollector{w: fileStats}
	InfoCounter := NewLogCounter(outLog)
	WarnCounter := NewLogCounter(outLog)
	ErrCounter := NewLogCounter(outLog)
	CritCounter := NewLogCounter(outLog)
	FatalCounter := NewLogCounter(outLog)
	makeLogHandlers(StatCollect, InfoCounter, WarnCounter, ErrCounter, CritCounter, FatalCounter)
	done <- true

	if !DevEnv {
		InfoLog.Println("Now logging stats with", PollCapacity, "records before flushing to disk, every", PollTime, "| Logging every", IntervalPoll)
	}
	var memStats runtime.MemStats
	var stat [PollCapacity]stats
	var pollCount int
	var cpuPercent Percent

	for range time.Tick(n) {
		s := &stat[pollCount]

		runtime.ReadMemStats(&memStats)
		vMem, _ := mem.VirtualMemory()

		if c, _ := cpu.CPUPercent(n, false); len(c) != 0 {
			cpuPercent = Percent(c[0] * 100)
		}
		s.CPUUsed = cpuPercent
		s.MemUsed = memStats.Alloc / KILOBYTE
		s.Memfree = 100 - Percent(vMem.UsedPercent)

		StatsLog.Println("Mem free:", s.Memfree, "% | Mem used:", s.MemUsed, "KB | CPU:", s.CPUUsed, "%")

		if pollCount == PollCapacity-1 {
			//fmt.Println(StatCollect.b.String())
			StatCollect.Flush()
			pollCount = -1
		}
		pollCount++
	}
}

func makeLogHandlers(statsHandle io.Writer,
	infoHandle io.Writer, warnHandle io.Writer, errorHandle io.Writer, critHandle io.Writer, fatalHandle io.Writer) {

	StatsLog = log.New(statsHandle, "STAT: ", log.Ldate|log.Ltime)

	flag := log.Ldate | log.Ltime | log.Lshortfile

	InfoLog = log.New(infoHandle, "INFO: ", flag)
	WarnLog = log.New(warnHandle, "WARN: ", flag)
	ErrLog = log.New(errorHandle, "ERR: ", flag)
	CritLog = log.New(critHandle, "CRIT: ", flag)
	FatalLog = log.New(fatalHandle, "FATAL: ", flag)
}

func LogInit() {
	var done = make(chan bool)
	go logPoll(IntervalPoll, done)
	<-done
}

func Log() {
	path := "/Images/png/sds.png"
	err := errors.New("Invalid image format")

	ErrGot(err, "Failed to fetch image", "path:", path)
	err = Err("Failed to process image", "path:", path)

	select {} // this will cause the program to run forever
}

func ErrGot(err error, s string, v ...interface{}) error {
	e := fmt.Sprintln(M, s, E, err, V, v)
	ErrLog.Output(2, e)
	return errors.New(s)
}

func Err(s string, v ...interface{}) error {
	e := fmt.Sprintln(M, s, V, v)
	ErrLog.Output(2, e)
	return errors.New(s)
}

package log

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"strconv"
	"time"

	"github.com/mikeqiao/newant/common"
	conf "github.com/mikeqiao/newant/config"
	"github.com/mikeqiao/newant/group"
)

// levels
const (
	DebugLevel = int(iota)
	ReleaseLevel
	ErrorLevel
	FatalLevel
	MaxLevel
)

const (
	printDebug   = "[debug] "
	printRelease = "[release] "
	printError   = "[error] "
	printFatal   = "[fatal] "
)

var gLogger, _ = New("", "", 0, log.LstdFlags|log.Llongfile)

type Logger struct {
	level      int
	baseLogger *log.Logger
	baseFile   *os.File

	strname  string
	pathname string
	flag     int
	day      int
	data     *common.Queue
	dowrite  chan int
	isClose  bool
}

func New(pathname, strname string, level, flag int) (*Logger, error) {
	//level
	if level >= MaxLevel {
		return nil, errors.New("unknown level")
	}

	//logger
	var baseLogger *log.Logger
	var baseFile *os.File
	var day = int(0)
	if pathname != "" {
		now := time.Now()
		filename := fmt.Sprintf("%d_%02d_%02d_%s.log",
			now.Year(),
			now.Month(),
			now.Day(),
			strname)
		file, err := os.OpenFile(path.Join(pathname, filename), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
		baseLogger = log.New(file, "", flag)
		baseFile = file
		day = now.Day()
	} else {
		baseLogger = log.New(os.Stdout, "", flag)
	}

	//new
	logger := new(Logger)
	logger.level = level
	logger.baseLogger = baseLogger
	logger.baseFile = baseFile

	logger.strname = strname
	logger.pathname = pathname
	logger.flag = flag
	logger.day = day
	logger.data = common.NewQueue()
	logger.dowrite = make(chan int)
	return logger, nil
}

func Debug(format string, a ...interface{}) {
	gLogger.doPrintf(DebugLevel, printDebug, format, a...)
}

func Release(format string, a ...interface{}) {
	gLogger.doPrintf(ReleaseLevel, printRelease, format, a...)
}

func Error(format string, a ...interface{}) {
	gLogger.doPrintf(ErrorLevel, printError, format, a...)
}

func Fatal(format string, a ...interface{}) {
	gLogger.doPrintf(FatalLevel, printFatal, format, a...)
}

func Close() {
	gLogger.isClose = true
}

func Export(logger *Logger) {
	if logger != nil {
		gLogger = logger
	}
}

func (l *Logger) Run() {
	for {
		if nil != l.data {
			value, ok := l.data.Pop().(bytes.Buffer)
			if ok {
				d := value.String()
				if "" != d {
					l.baseLogger.Output(3, d)
				} else {
					fmt.Println("d is nil ")
				}
			}
		}
		if l.isClose && 0 == l.data.Len() {
			goto Loop
		}
	}
Loop:
	fmt.Println("log close")
	l.Close()
	group.Done()
}

func (l *Logger) ChangeFile() {
	old := l.baseFile

	//	l.baseFile = nil
	//	l.baseLogger = nil

	if l.pathname != "" {
		now := time.Now()

		filename := fmt.Sprintf("%d_%02d_%02d_%s.log",
			now.Year(),
			now.Month(),
			now.Day(),
			l.strname)

		file, err := os.OpenFile(path.Join(l.pathname, filename), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
		}
		l.baseLogger = log.New(file, "", l.flag)
		l.baseFile = file

		if old != nil {
			old.Close()
		}
	} else {
		l.baseLogger = log.New(os.Stdout, "", l.flag)
	}

}

// It's dangerous to call the methodd on logging
func (l *Logger) Close() {
	if l.baseFile != nil {
		l.baseFile.Close()
	}
	if nil != l.data {
		l.data.Close()
	}
	l.baseFile = nil
	l.baseLogger = nil
}

func (l *Logger) doPrintf(level int, printLevel string, format string, a ...interface{}) {
	if level < l.level {
		return
	}
	now := time.Now()
	if 0 != l.day && now.Day() != l.day {
		l.day = now.Day()
		l.ChangeFile()
	}
	if l.baseLogger == nil {
		panic("logger closed")
	}

	format = printLevel + format
	t := time.Now().String()
	var buffer bytes.Buffer
	buffer.WriteString(t[:26])
	_, file, line, ok := runtime.Caller(2)
	if ok {
		buffer.WriteString(" ")
		buffer.WriteString(file)
		buffer.WriteString("	line:")
		buffer.WriteString(strconv.Itoa(line))
		buffer.WriteString(":")
	}
	buffer.WriteString(fmt.Sprintf(format, a...))
	l.data.Push(buffer)
	if level == FatalLevel {
		os.Exit(1)
	}
}

func Init() {
	if conf.Config.LogLevel < MaxLevel {
		logger, err := New(conf.Config.LogPath, conf.Config.ServiceInfo.ServerName, conf.Config.LogLevel, conf.Config.LogFlag)
		if err != nil {
			panic(err)
		}
		Export(logger)
	}
}

func Run() {
	group.Add(1)
	go gLogger.Run()

}

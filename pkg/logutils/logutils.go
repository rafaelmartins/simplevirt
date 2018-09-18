package logutils

import (
	"log"
	"log/syslog"
	"os"
)

var (
	Crit    = log.New(os.Stderr, "[CRITICAL] ", log.LstdFlags)
	Error   = log.New(os.Stderr, "[ERROR]    ", log.LstdFlags)
	Warning = log.New(os.Stderr, "[WARNING]  ", log.LstdFlags)
	Notice  = log.New(os.Stderr, "[NOTICE]   ", log.LstdFlags)

	syslogMap = map[*log.Logger]syslog.Priority{
		Crit:    syslog.LOG_CRIT,
		Error:   syslog.LOG_ERR,
		Warning: syslog.LOG_WARNING,
		Notice:  syslog.LOG_NOTICE,
	}
)

func UseSyslog(tag string) error {
	for logger, prio := range syslogMap {
		logger.SetFlags(0)
		logger.SetPrefix("")
		writer, err := syslog.New(syslog.LOG_DAEMON|prio, tag)
		if err != nil {
			return err
		}
		logger.SetOutput(writer)
	}
	return nil
}

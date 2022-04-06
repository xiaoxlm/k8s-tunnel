package log

import (
	"github.com/sirupsen/logrus"
	"os"
)

func LogInit(projectName string, level logrus.Level) {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	logrus.AddHook(&LogHook{Name:projectName})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(level)
}

type LogHook struct {
	Name string
}

func (hook *LogHook) Fire(entry *logrus.Entry) error {
	entry.Data["project"] = hook.Name

	return nil
}

func (hook *LogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

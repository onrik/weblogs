package main

import (
	"bufio"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"os"
	"time"
)

const (
	BUFFER_SIZE      = 50
	READ_BUFFER_SIZE = 16
	CHECK_TIME       = 500 * time.Millisecond
)

type Reader struct {
	filename   string
	lastModify time.Time
	position   int64
	channel    chan string
	Buffer     []string
}

func (self *Reader) read() {
	file, err := os.Open(self.filename)
	if err != nil {
		log.WithFields(log.Fields{"file": self.filename, "error": err}).Fatal("Open file error")
	}

	if self.position > 0 {
		file.Seek(self.position, 0)
	}
	defer file.Close()

	reader := bufio.NewReaderSize(file, READ_BUFFER_SIZE)
	for {
		line, err := reader.ReadString('\n')
		if line != "" && line[0] == '{' {
			l := fmt.Sprintf(`{"position": %d, "data": %s}`, self.getPosition(file), line)

			self.channel <- l

			self.Buffer = append(self.Buffer, l)
			if len(self.Buffer) > BUFFER_SIZE {
				self.Buffer = self.Buffer[len(self.Buffer)-BUFFER_SIZE:]
			}
		}

		if err != nil {
			break
		}
	}

	self.savePosition(file)
}

func (self *Reader) savePosition(file *os.File) {
	var err error
	self.position, err = file.Seek(0, os.SEEK_CUR)
	if err != nil {
		log.WithField("error", err).Error("Get position error")
	}
}

func (self *Reader) getPosition(file *os.File) int64 {
	position, err := file.Seek(0, os.SEEK_CUR)
	if err != nil {
		log.WithField("error", err).Error("Get position error")
	}
	return position
}

func (self *Reader) GetHistory(position int64) ([]string, int64, error) {
	log.WithField("position", position).Info("Get history")
	result := []string{}

	file, err := os.Open(self.filename)
	if err != nil {
		log.WithFields(log.Fields{"file": self.filename, "error": err}).Fatal("Open file error")
		return result, position, err
	}

	newPosition, _ := file.Seek(max(position-5000, 0), 0)
	defer file.Close()
	log.WithField("pos", newPosition).Debug("New position")

	reader := bufio.NewReaderSize(file, READ_BUFFER_SIZE)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if line == "" || line[0] != '{' {
			continue
		}
		result = append(result, line)

		log.WithFields(log.Fields{"line": line, "postition": self.getPosition(file)}).Debug("History loaded")
		if self.getPosition(file) >= position {
			break
		}

	}

	pos, _ := file.Seek(0, os.SEEK_CUR)
	log.WithFields(log.Fields{"postition": pos}).Info("History loaded")

	return result, newPosition, nil
}

func (self *Reader) Watch() {
	ticker := time.NewTicker(CHECK_TIME)
	for {
		info, err := os.Stat(self.filename)
		if err != nil {
			log.Errorf("Open file error")
		}
		if info.ModTime().After(self.lastModify) {
			self.lastModify = info.ModTime()
			self.read()
		}
		<-ticker.C
	}
}

func NewReader(filename string, channel chan string) *Reader {
	reader := Reader{
		filename: filename,
		channel:  channel,
		Buffer:   []string{},
	}

	reader.read()
	info, err := os.Stat(filename)
	if err != nil {
		log.WithField("error", err).Error("Open file error")
	}
	reader.lastModify = info.ModTime()
	log.Info("File loaded")

	return &reader
}

func max(a, b int64) int64 {
	if a > b {
		return a
	} else {
		return b
	}
}

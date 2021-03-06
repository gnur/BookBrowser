package main

import (
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gnur/booksing"
	"github.com/sirupsen/logrus"
)

type booksingApp struct {
	db           database
	mqttClient   mqtt.Client
	bookDir      string
	importDir    string
	logger       *logrus.Entry
	timezone     *time.Location
	adminUser    string
	cfg          configuration
	state        string
	bookQ        chan string
	resultQ      chan parseResult
	searchQ      chan booksing.Book
	saveInterval time.Duration
}

type parseResult int32

// hold all possible book parse results
const (
	OldBook       parseResult = iota
	AddedBook     parseResult = iota
	DuplicateBook parseResult = iota
	InvalidBook   parseResult = iota
	DBErrorBook   parseResult = iota
)

type database interface {
	AddDownload(booksing.Download) error
	GetDownloads(int) ([]booksing.Download, error)

	SaveUser(*booksing.User) error
	GetUser(string) (booksing.User, error)

	GetUsers() ([]booksing.User, error)

	GetBookCount() int
	UpdateBookCount(int) error
	GetBookCountHistory(time.Time, time.Time) ([]booksing.BookCount, error)

	AddHash(string) error
	HasHash(string) (bool, error)

	Close()

	AddBooks([]booksing.Book, bool) error
	GetBook(string) (*booksing.Book, error)
	DeleteBook(string) error
	GetBooks(string, int64, int64) (*booksing.SearchResult, error)
}

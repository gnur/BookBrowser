package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gnur/booksing"
	"github.com/gnur/booksing/storm"
	"github.com/markbates/pkger"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kelseyhightower/envconfig"
	log "github.com/sirupsen/logrus"
)

// V is the holder struct for all possible template values
type V struct {
	Results    int64
	Error      error
	Books      []booksing.Book
	Book       *booksing.Book
	Users      []booksing.User
	Downloads  []booksing.Download
	Q          string
	TimeTaken  int
	Stats      []booksing.BookCount
	IsAdmin    bool
	Username   string
	TotalBooks int
	Limit      int64
	Offset     int64
	Indexing   bool
}

type configuration struct {
	AdminUser     string `default:"unknown"`
	UserHeader    string `default:""`
	AllowAllusers bool   `default:"true"`
	BookDir       string `default:"."`
	ImportDir     string `default:"./import"`
	FailDir       string `default:"./failed"`
	DatabaseDir   string `default:"./db/"`
	LogLevel      string `default:"info"`
	BindAddress   string `default:":7132"`
	Timezone      string `default:"Europe/Amsterdam"`
	MQTTEnabled   bool   `default:"false"`
	MQTTTopic     string `default:"events"`
	MQTTHost      string `default:"tcp://localhost:1883"`
	MQTTClientID  string `default:"booksing"`
	BatchSize     int    `default:"50"`
	Workers       int    `default:"5"`
	SaveInterval  string `default:"10s"`
}

func main() {
	var cfg configuration
	err := envconfig.Process("booksing", &cfg)
	if err != nil {
		log.WithField("err", err).Fatal("Could not parse full config from environment")
	}

	logLevel, err := log.ParseLevel(cfg.LogLevel)
	if err == nil {
		log.SetLevel(logLevel)
	}
	if cfg.ImportDir == "" {
		cfg.ImportDir = path.Join(cfg.BookDir, "import")
	}

	var db database
	log.WithField("dbpath", cfg.DatabaseDir).Debug("using this file")
	db, err = storm.New(cfg.DatabaseDir)
	if err != nil {
		log.WithField("err", err).Fatal("could not create fileDB")
	}
	defer db.Close()

	interval, err := time.ParseDuration(cfg.SaveInterval)
	if err != nil {
		interval = 10 * time.Second
	}

	tz, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		log.WithField("err", err).Fatal("could not load timezone")
	}

	tpl := template.New("")
	tpl.Funcs(templateFunctions)

	err = pkger.Walk("/cmd/ui/templates", func(path string, info os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".html") {
			log.WithField("path", path).Debug("loading template")
			f, err := pkger.Open(path)
			if err != nil {
				return err
			}
			sl, err := ioutil.ReadAll(f)
			if err != nil {
				return err
			}
			_, err = tpl.Parse(string(sl))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.WithField("err", err).Fatal("could not read templates")
		return
	}

	app := booksingApp{
		db:           db,
		bookDir:      cfg.BookDir,
		importDir:    cfg.ImportDir,
		timezone:     tz,
		adminUser:    cfg.AdminUser,
		logger:       log.WithField("app", "booksing"),
		cfg:          cfg,
		bookQ:        make(chan string),
		resultQ:      make(chan parseResult),
		searchQ:      make(chan booksing.Book),
		saveInterval: interval,
	}

	if app.cfg.MQTTEnabled {
		mqttClient, err := newMQTTClient(cfg.MQTTHost, cfg.MQTTClientID)
		if err != nil {
			log.WithField("err", err).Fatal("Unable to connect to mqtt")
			return
		}
		app.mqttClient = mqttClient
	}

	if cfg.ImportDir != "" {
		go app.refreshLoop()
		for w := 0; w < 5; w++ { //not sure yet how concurrent-proof my solution is
			go app.bookParser()
		}
		go app.resultParser()
		go app.searchUpdater()
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(Logger(app.logger), gin.Recovery())
	r.SetHTMLTemplate(tpl)

	static := r.Group("/", func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=86400, immutable")
	})
	static.StaticFS("/static", pkger.Dir("/cmd/ui/static"))

	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	r.GET("/kill", func(c *gin.Context) {
		app.logger.Fatal("Killing so I get restarted anew")
	})

	r.GET("/status", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": app.state,
		})
	})

	auth := r.Group("/")
	auth.Use(app.BearerTokenMiddleware())
	{
		auth.GET("/", app.search)
		auth.GET("/bookmarks", app.bookmarks)
		auth.GET("/rotateShelve/:hash", app.rotateIcon)
		auth.POST("/rotateShelve/:hash", app.rotateIcon)
		auth.GET("/download", app.downloadBook)
		auth.GET("/icons/:hash", app.serveIcon)

	}

	admin := r.Group("/admin")
	admin.Use(gin.Recovery(), app.BearerTokenMiddleware(), app.mustBeAdmin())
	{
		admin.GET("/users", app.showUsers)
		admin.GET("/stats", app.showStats)
		admin.GET("/downloads", app.showDownloads)
		admin.POST("/delete/:hash", app.deleteBook)
		admin.POST("user/:username", app.updateUser)
		admin.POST("/adduser", app.addUser)
	}

	log.Info("booksing is now running")
	port := os.Getenv("PORT")

	if port == "" {
		port = cfg.BindAddress
	} else {
		port = fmt.Sprintf(":%s", port)
	}

	err = r.Run(port)
	if err != nil {
		log.WithField("err", err).Fatal("unable to start running")
	}
}

func (app *booksingApp) IsUserAdmin(c *gin.Context) bool {

	return true
}

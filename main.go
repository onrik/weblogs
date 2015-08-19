package main

import (
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"strconv"
)

var reader *Reader
var hub *ConnectionsHub

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func indexHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", nil)
}

func historyHandler(c *gin.Context) {
	position, err := strconv.ParseInt(c.Request.FormValue("position"), 10, 64)
	if err != nil {
		log.WithFields(log.Fields{"position": c.Request.FormValue("position"), "error": err.Error()}).Error("Get history error")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logs, newPosition, err := reader.GetHistory(position)
	if err != nil {
		log.WithField("error", err).Error("Get history error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := map[string]interface{}{
		"data":     logs,
		"position": newPosition,
	}

	c.JSON(http.StatusOK, response)
}

func socketHandler(c *gin.Context) {
	socket, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.WithFields(log.Fields{"error": err.Error(), "headers": c.Request.Header}).Error("Connect error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	conn := NewConnection(socket)
	for i := range reader.Buffer {
		conn.Send <- reader.Buffer[i]
	}

	hub.Register <- conn
	go conn.writePump()
	conn.readPump()
}

func main() {
	filename := flag.String("f", "", "logs file")
	port := flag.Int("port", 8000, "server port")
	debug := flag.Bool("debug", false, "debug mode")
	flag.Parse()

	if debug != nil && *debug == false {
		gin.SetMode(gin.ReleaseMode)
	}

	hub = &ConnectionsHub{
		Connections: map[*Connection]bool{},
		Send:        make(chan string, 128),
		Register:    make(chan *Connection),
		Unregister:  make(chan *Connection),
	}
	go hub.run()

	reader = NewReader(*filename, hub.Send)
	go reader.Watch()

	server := gin.Default()
	server.LoadHTMLGlob("templates/*")

	server.GET("/", indexHandler)
	server.GET("/sock/", socketHandler)
	server.GET("/history/", historyHandler)

	server.Run(fmt.Sprintf(":%d", *port))
}

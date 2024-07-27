package store

import (
	"ChatTwo/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"sync"
	"time"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]string)
var broadcast = make(chan models.Message, 256)
var pubClient = make(chan *websocket.Conn, 256)
var curRoomID = make(chan string, 256)
var clientsMutex sync.Mutex

func main() {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.GET("/ws/:roomId", handleClient)

	go handleMessages()

	r.Run(":15421")
}

func handleMessages() {
	for {
		select {
		case msg := <-broadcast:
			publisher := <-pubClient
			room := <-curRoomID
			clientsMutex.Lock()
			for client := range clients {
				if publisher != client && clients[client] == room {
					log.Println("Client[client]: ", clients[client])
					err := client.WriteJSON(msg)
					if err != nil {
						log.Println("write:", err)
						client.Close()
						delete(clients, client)
					}
				}
			}
			clientsMutex.Unlock()
		}
	}
}

func handleClient(c *gin.Context) {
	roomId := c.Param("roomId")
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {
			log.Println(err)
		}
	}(conn)

	log.Println(conn.RemoteAddr().String())

	clientsMutex.Lock()
	clients[conn] = roomId
	clientsMutex.Unlock()

	for {
		var msg models.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println(err)
			clientsMutex.Lock()
			delete(clients, conn)
			clientsMutex.Unlock()
			log.Println("Handle Connections: ", clients)
			return
		}
		log.Println("Message Sent")
		msg.PubTime = time.Now()
		broadcast <- msg
		pubClient <- conn
		curRoomID <- roomId
	}
}

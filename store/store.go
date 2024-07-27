package store

import (
	"ChatTwo/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

var updrager = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clients = make(map[*websocket.Conn]string)
var broadcast = make(chan models.Message)

// var pubClient = make(chan *websocket.Conn)
//var curRoomID = make(chan string)

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
		log.Println("Message Received")

		//publisher := <-pubClient
		log.Println(clients)
		msg := <-broadcast
		for client := range clients {
			//if publisher != client {
			//room := <-curRoomID
			//if clients[client] == room {
			log.Println("Client[client]: ", clients[client])
			//log.Println("curRoomID: ", room)
			err := client.WriteJSON(msg)
			if err != nil {
				log.Println("write:", err)
				delete(clients, client)
			}
		}
		//}
	}
}

func handleClient(c *gin.Context) {

	roomId := c.Param("roomId")
	//
	conn, err := updrager.Upgrade(c.Writer, c.Request, nil)
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

	clients[conn] = roomId
	for {
		var msg models.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println(err)
			delete(clients, conn)
			log.Println("Handle Connections: ", clients)
		}
		log.Println("Message Sent")
		//msg.PubTime = time.Now().Add(5*time.Hour + 30*time.Minute)
		msg.PubTime = time.Now()
		broadcast <- msg
		//pubClient <- conn
		//curRoomID <- roomId
	}
}

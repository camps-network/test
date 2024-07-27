package main

import (
	"ChatTwo/models"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/streadway/amqp"
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
var curUser = make(chan *websocket.Conn, 256)
var clientsMutex sync.Mutex

var rabbitMQConnection *amqp.Connection
var rabbitMQChannel *amqp.Channel

const (
	exchangeName = "chat_exchange"
	queueName    = "chat_queue"
)

func main() {
	// Connect to RabbitMQ
	var err error
	rabbitMQConnection, err = amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitMQConnection.Close()

	rabbitMQChannel, err = rabbitMQConnection.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer rabbitMQChannel.Close()

	err = rabbitMQChannel.ExchangeDeclare(
		exchangeName, // name
		"fanout",     // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare an exchange: %v", err)
	}

	_, err = rabbitMQChannel.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare a queue: %v", err)
	}

	err = rabbitMQChannel.QueueBind(
		queueName,    // queue name
		"",           // routing key
		exchangeName, // exchange
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to bind a queue: %v", err)
	}

	// Consume messages from RabbitMQ
	msgs, err := rabbitMQChannel.Consume(
		queueName, // queue
		"",        // consumer
		true,      // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	go func() {
		for d := range msgs {
			var msg models.Message
			err := json.Unmarshal(d.Body, &msg)
			if err != nil {
				log.Printf("Error decoding JSON: %v", err)
				continue
			}
			broadcast <- msg
		}
	}()

	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	r.GET("/ws/:roomId", handleClient)

	go handleMessages()

	r.Run("192.168.0.106:15421")
}

func handleMessages() {
	for msg := range broadcast {
		clientsMutex.Lock()
		curUserIDconn := <-curUser
		for client, room := range clients {
			if msg.RoomID == room && curUserIDconn != client {
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
			return
		}
		log.Printf("Received message: %+v\n", msg)

		msg.PubTime = time.Now()
		msg.RoomID = roomId // Ensure the message has the RoomID
		body, err := json.Marshal(msg)
		if err != nil {
			log.Println("Error marshalling JSON: ", err)
			continue
		}
		clientsMutex.Lock()
		curUser <- conn
		clientsMutex.Unlock()

		err = rabbitMQChannel.Publish(
			exchangeName, // exchange
			"",           // routing key
			false,        // mandatory
			false,        // immediate
			amqp.Publishing{
				ContentType: "application/json",
				Body:        body,
			})
		if err != nil {
			log.Println("Failed to publish a message: ", err)
		}
	}
}

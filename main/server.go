package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/gorilla/websocket"
)

// upgradeConnection is the websocket upgrader from gorilla/websockets
var upgradeConnection = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var clients = make(map[*websocket.Conn]string)

var tickets = make(map[int]Client)

var wsChan = make(chan WsRequestMessage)

type Client struct {
	Username string
	Conn     *websocket.Conn
}

type WsRequestMessage struct {
	Action   string          `json:"action"`
	Username string          `json:"username"`
	Ticket   int             `json:"ticket"`
	Conn     *websocket.Conn `json:"-"`
}

type WsResponseMessage struct {
	Action      string `json:"action"`
	Message     string `json:"message"`
	OnlineUsers string `json:"onlineUsers"`
	TicketList  string `json:"ticketList"`
}

func WsEndPoint(w http.ResponseWriter, r *http.Request) {
	// upgrade this connection to a websocket
	conn, err := upgradeConnection.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}
	log.Println("New client is connected via Websocket!")

	clients[conn] = ""

	var response WsResponseMessage
	response.TicketList = getTicketList()
	response.OnlineUsers = getOnlineUsers()
	err = conn.WriteJSON(response)
	if err != nil {
		log.Println(err)
	}

	ListenToWebSocket(conn)
}

func ListenToWebSocket(conn *websocket.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Error", fmt.Sprintf("%v", r))
		}
	}()

	var requestMessage WsRequestMessage
	for {
		err := conn.ReadJSON(&requestMessage)
		if err != nil {
			log.Println(err)
		}

		requestMessage.Conn = conn
		wsChan <- requestMessage
	}
}

func ListenToChannel() {
	var response WsResponseMessage
	for {
		// update username
		requestMessage := <-wsChan

		switch requestMessage.Action {
		case "left":
			delete(clients, requestMessage.Conn)
			delete(tickets, requestMessage.Ticket)
			response.TicketList = getTicketList()
			response.OnlineUsers = getOnlineUsers()
			broadcastToAll(response)
			log.Println("A user have left!")
		case "username":
			// send online userList
			clients[requestMessage.Conn] = requestMessage.Username
			response.TicketList = getTicketList()
			response.OnlineUsers = getOnlineUsers()
			response.Action = "username"
			broadcastToAll(response)
		case "ticket":
			var client Client
			if requestMessage.Username != "" && requestMessage.Conn != nil {
				client = Client{requestMessage.Username, requestMessage.Conn}
			} else {
				client = Client{}
			}
			tickets[requestMessage.Ticket] = client
			response.Action = "ticket"
			response.OnlineUsers = getOnlineUsers()
			response.TicketList = getTicketList()
			broadcastToAll(response)
		}

	}
}

func getOnlineUsers() string {
	var onlineUsers string
	for _, x := range clients {
		if x != "" {
			onlineUsers += x + `<br>`
		}
	}
	return onlineUsers
}

func getTicketList() string {
	var ticketList string
	for id, client := range tickets {
		if (client != Client{}) {
			ticketList += strconv.Itoa(id) + " : " + client.Username + `<br>`
		} else {
			ticketList += strconv.Itoa(id) + ` : nicht zugewiesen <br>`
		}
	}
	return ticketList
}

func broadcastToAll(response WsResponseMessage) {
	for client := range clients {
		err := client.WriteJSON(response)
		if err != nil {
			log.Println("Websocket Error!")
			_ = client.Close()
			delete(clients, client)

			// update online users list
			response.OnlineUsers = getOnlineUsers()
			broadcastToAll(response)
		}
	}
}

func Home(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Home Page")
}

func main() {
	http.HandleFunc("/", Home)
	http.HandleFunc("/ws", WsEndPoint)

	go cmd()

	log.Println("Start listening to Channel")
	go ListenToChannel()

	log.Println("Start listening on port 8080!")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func cmd() {
	for {
		var char string
		var requestMessage WsRequestMessage
		log.Println("Enter 'n' to create new ticket!")
		fmt.Scanf("%s", &char)
		if char == "n" {
			log.Println("New ticket was created!")
			requestMessage.Action = "ticket"
			requestMessage.Ticket = rand.Intn(10000)
			wsChan <- requestMessage
			log.Println("Ticket was sent!")
		} else {
			log.Printf("Unknow char: %s", char)
		}

	}
}

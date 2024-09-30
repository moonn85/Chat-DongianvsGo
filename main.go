package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type Message struct {
	Sender  string `json:"sender"`
	Content string `json:"content"`
	Time    string `json:"time"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)

func saveMessageToDB(msg Message) {
	query := `INSERT INTO messages (sender, content, time) VALUES (?, ?, ?)`
	_, err := db.Exec(query, msg.Sender, msg.Content, msg.Time)
	if err != nil {
		fmt.Println("Error saving message to DB:", err)
	}
}
func loadChatHistory(ws *websocket.Conn) {
	rows, err := db.Query("SELECT sender, content, time FROM messages")
	if err != nil {
		fmt.Println("Error loading chat history:", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var msg Message
		err := rows.Scan(&msg.Sender, &msg.Content, &msg.Time)
		if err != nil {
			fmt.Println("Error scanning message:", err)
			continue
		}
		ws.WriteJSON(msg)
	}
}
func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ws.Close()

	clients[ws] = true

	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			fmt.Println("Error reading message:", err)
			delete(clients, ws)
			break
		}

		msg.Time = time.Now().Format("15:04:05")
		broadcast <- msg

		// Lưu tin nhắn vào cơ sở dữ liệu
		saveMessageToDB(msg)
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				fmt.Println("Error writing message:", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}

var db *sql.DB

func initDatabase() {
	var err error
	db, err = sql.Open("sqlite3", "./chat.db")
	if err != nil {
		panic(err)
	}

	// Tạo bảng lưu trữ tin nhắn nếu chưa có
	createTableQuery := `
    CREATE TABLE IF NOT EXISTS messages (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        sender TEXT,
        content TEXT,
        time TEXT
    );
    `
	_, err = db.Exec(createTableQuery)
	if err != nil {
		panic(err)
	}
}

func main() {
	fs := http.FileServer(http.Dir("./"))
	http.Handle("/", fs)

	http.HandleFunc("/ws", handleConnections)

	go handleMessages()

	fmt.Println("Server started at :8082")
	err := http.ListenAndServe(":8082", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

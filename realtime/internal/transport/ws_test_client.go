package transport

import (
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// отправляем auth первым сообщением
	auth := `{"type":"auth","token":"<ACCESS_TOKEN>"}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(auth)); err != nil {
		log.Fatal(err)
	}

	// отправляем несколько keystroke
	for i := 0; i < 5; i++ {
		msg := fmt.Sprintf(`{"type":"keystroke","char":"a","t":%d}`, i*100)
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			log.Fatal(err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// читаем ответы 1s
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	for {
		_, m, err := conn.ReadMessage()
		if err != nil {
			break
		}
		fmt.Println("recv:", string(m))
	}
}

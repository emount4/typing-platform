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
	auth := `{"type":"auth","v":1,"token":"<ACCESS_TOKEN>","anon_id":null}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(auth)); err != nil {
		log.Fatal(err)
	}

	msg := fmt.Sprintf(`{"type":"keystroke.batch","seq":0,"items":[{"char":"a","t":0},{"char":"b","t":100},{"char":"c","t":200}]}`)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		log.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

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

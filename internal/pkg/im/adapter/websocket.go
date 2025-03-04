package adapter

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WsAdapter Websocket 适配器
type WsAdapter struct {
	conn *websocket.Conn
}

func NewWsAdapter(w http.ResponseWriter, r *http.Request) (*WsAdapter, error) {

	upGrader := websocket.Upgrader{
		ReadBufferSize:  1024 * 2, // 指定读缓存区大小
		WriteBufferSize: 1024 * 2, // 指定写缓存区大小
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upGrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	return &WsAdapter{conn: conn}, nil
}

func (w *WsAdapter) Network() string {
	return WssType
}

func (w *WsAdapter) Read() ([]byte, error) {

	_, content, err := w.conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	return content, nil
}

func (w *WsAdapter) Write(bytes []byte) error {

	err := w.conn.SetWriteDeadline(time.Now().Add(10 * time.Millisecond))
	if err != nil {
		return err
	}

	return w.conn.WriteMessage(websocket.TextMessage, bytes)
}

func (w *WsAdapter) Close() error {
	return w.conn.Close()
}

func (w *WsAdapter) SetCloseHandler(fn func(code int, text string) error) {
	w.conn.SetCloseHandler(fn)
}

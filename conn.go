//    Title: conn.go
//    Author: Jon Cody
//
//    This program is free software: you can redistribute it and/or modify
//    it under the terms of the GNU General Public License as published by
//    the Free Software Foundation, either version 3 of the License, or
//    (at your option) any later version.
//
//    This program is distributed in the hope that it will be useful,
//    but WITHOUT ANY WARRANTY; without even the implied warranty of
//    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//    GNU General Public License for more details.
//
//    You should have received a copy of the GNU General Public License
//    along with this program.  If not, see <http://www.gnu.org/licenses/>.

package wsrooms

import (
	"net/http"
	"sync"
	"time"

	"github.com/chuckpreslar/emission"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Conn type represents a single client
type Conn struct {
	Cookie *sync.Map
	Socket *websocket.Conn
	ID     string
	Send   chan []byte
	Rooms  *sync.Map
}

// CookieReader reads cookie
type CookieReader func(*http.Request) *sync.Map

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = pongWait * 9 / 10
	maxMessageSize = 1024 * 1024 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

var (
	// ConnManager stores all Conn types by their uuid
	ConnManager = struct {
		Conns *sync.Map
	}{
		Conns: new(sync.Map),
	}
	// Emitter emits received Messages with non-reserved event names
	Emitter = emission.NewEmitter()
)

// HandleData handles incoming, error free messages
func HandleData(c *Conn, msg *Message) {
	switch msg.Event {
	case "join":
		c.Join(msg.Room)
	case "leave":
		c.Leave(msg.Room)
	case "joined":
		c.Emit(msg)
	case "left":
		c.Emit(msg)
		room, ok := RoomManager.Rooms.Load(msg.Room)
		if !ok {
			break
		}
		room.(*Room).Members.Delete(c.ID)
		var length int
		room.(*Room).Members.Range(func(_, _ interface{}) bool {
			length++
			return true
		})
		if length == 0 {
			room.(*Room).Stop()
		}
	default:
		if msg.Dst != "" {
			room, rok := RoomManager.Rooms.Load(msg.Room)
			if rok == false {
				break
			}
			id, mok := room.(*Room).Members.Load(msg.Dst)
			if mok == false {
				break
			}
			dst, cok := ConnManager.Conns.Load(id)
			if cok == false {
				break
			}
			dst.(*Conn).Send <- msg.Bytes()
		} else if Emitter.GetListenerCount(msg.Event) > 0 {
			Emitter.Emit(msg.Event, c, msg)
		} else {
			c.Emit(msg)
		}
	}
}

// ReadPump is loop for reading
func (c *Conn) ReadPump() {
	defer func() {
		c.Rooms.Range(func(k, v interface{}) bool {
			room, ok := RoomManager.Rooms.Load(k.(string))
			if ok {
				room.(*Room).Leave(c)
			}
			return true
		})
		c.Socket.Close()
	}()
	c.Socket.SetReadLimit(maxMessageSize)
	c.Socket.SetReadDeadline(time.Now().Add(pongWait))
	c.Socket.SetPongHandler(func(string) error {
		c.Socket.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, data, err := c.Socket.ReadMessage()
		if err != nil {
			if _, wok := err.(*websocket.CloseError); wok == false {
				break
			}
			c.Rooms.Range(func(k, v interface{}) bool {
				room, rok := RoomManager.Rooms.Load(k.(string))
				if !rok {
					return true
				}
				room.(*Room).Emit(c, ConstructMessage(k.(string), "left", "", c.ID, []byte(c.ID)))
				room.(*Room).Members.Delete(c.ID)
				var length int
				room.(*Room).Members.Range(func(_, _ interface{}) bool {
					length++
					return true
				})
				if length == 0 {
					room.(*Room).Stop()
				}
				return true
			})
			break
		}
		HandleData(c, BytesToMessage(data))
	}
}

func (c *Conn) write(mt int, payload []byte) error {
	c.Socket.SetWriteDeadline(time.Now().Add(writeWait))
	return c.Socket.WriteMessage(mt, payload)
}

// WritePump is loop for writing
func (c *Conn) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Socket.Close()
	}()
	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

// Join adds the Conn to a Room. If the Room does not exist, it is created
func (c *Conn) Join(name string) {
	room, ok := RoomManager.Rooms.Load(name)
	if !ok {
		room = NewRoom(name)
	}
	c.Rooms.Store(name, name)
	room.(*Room).Join(c)
}

// Leave removes the Conn from a Room
func (c *Conn) Leave(name string) {
	room, rok := RoomManager.Rooms.Load(name)
	if !rok {
		return
	}
	_, cok := c.Rooms.Load(name)
	if !cok {
		return
	}
	c.Rooms.Delete(name)
	room.(*Room).Leave(c)
}

// Emit broadcasts a Message to all members of a Room
func (c *Conn) Emit(msg *Message) {
	room, ok := RoomManager.Rooms.Load(msg.Room)
	if ok {
		room.(*Room).Emit(c, msg)
	}
}

// NewConnection upgrades an HTTP connection and creates a new Conn type
func NewConnection(w http.ResponseWriter, r *http.Request, cr CookieReader) *Conn {
	socket, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return nil
	}
	c := &Conn{
		Socket: socket,
		ID:     id.String(),
		Send:   make(chan []byte, 256),
		Rooms:  new(sync.Map),
	}
	if cr != nil {
		c.Cookie = cr(r)
	}
	ConnManager.Conns.Store(c.ID, c)
	return c
}

// SocketHandler calls NewConnection, starts the returned Conn's writer, joins the root room, and finally starts the Conn's reader
func SocketHandler(cr CookieReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		c := NewConnection(w, r, cr)
		if c != nil {
			go c.WritePump()
			c.Join("root")
			go c.ReadPump()
		}
	}
}

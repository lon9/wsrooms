//    Title: room.go
//    Author: Jonanthan David Cody
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
	"encoding/json"
	"log"
	"sync"
)

// Room type represents a communication channel
type Room struct {
	Name      string
	Members   *sync.Map
	stopchan  chan bool
	joinchan  chan *Conn
	leavechan chan *Conn
	Send      chan *RoomMessage
}

// RoomManager stores all Room types by their name
var RoomManager = struct {
	Rooms *sync.Map
}{
	Rooms: new(sync.Map),
}

// Start starts the Room
func (r *Room) Start() {
	for {
		select {
		case c := <-r.joinchan:
			members := make([]string, 0)
			r.Members.Range(func(k interface{}, v interface{}) bool {
				members = append(members, k.(string))
				return true
			})
			r.Members.Store(c.ID, c.ID)
			payload, err := json.Marshal(members)
			if err != nil {
				log.Println(err)
				break
			}
			c.Send <- ConstructMessage(r.Name, "join", "", c.ID, payload).Bytes()
		case c := <-r.leavechan:
			id, ok := r.Members.Load(c.ID)
			if !ok {
				break
			}
			r.Members.Delete(id)
			c.Send <- ConstructMessage(r.Name, "leave", "", id.(string), []byte(c.ID)).Bytes()
		case rmsg := <-r.Send:
			r.Members.Range(func(k interface{}, v interface{}) bool {
				c, ok := ConnManager.Conns.Load(k)
				if !ok || c == rmsg.Sender {
					return true
				}
				select {
				case c.(*Conn).Send <- rmsg.Data:
				default:
					r.Members.Delete(k)
					close(c.(*Conn).Send)
				}
				return true
			})
		case <-r.stopchan:
			RoomManager.Rooms.Delete(r.Name)
			return
		}
	}
}

// Stop stops the Room
func (r *Room) Stop() {
	r.stopchan <- true
}

// Join adds a Conn to the Room
func (r *Room) Join(c *Conn) {
	r.joinchan <- c
}

// Leave removes a Conn from the Room
func (r *Room) Leave(c *Conn) {
	r.leavechan <- c
}

// Emit broadcasts data to all members of the Room
func (r *Room) Emit(c *Conn, msg *Message) {
	r.Send <- &RoomMessage{c, msg.Bytes()}
}

// NewRoom creates a new Room type and starts it
func NewRoom(name string) *Room {
	r := &Room{
		Name:      name,
		Members:   new(sync.Map),
		stopchan:  make(chan bool),
		joinchan:  make(chan *Conn),
		leavechan: make(chan *Conn),
		Send:      make(chan *RoomMessage),
	}
	RoomManager.Rooms.Store(name, r)
	go r.Start()
	return r
}

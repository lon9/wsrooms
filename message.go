//    Title: message.go
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
	"encoding/json"
)

// Message protocol followed by wsrooms servers and clients
type Message struct {
	RoomLength    int    `json:"roomLength"`    // room name length
	Room          string `json:"room"`          // room name
	EventLength   int    `json:"eventLength"`   // event name length
	Event         string `json:"event"`         // event name
	DstLength     int    `json:"dstLength"`     // destination id length
	Dst           string `json:"dst"`           // destination id
	SrcLength     int    `json:"srcLength"`     // source id length
	Src           string `json:"src"`           // source id
	PayloadLength int    `json:"payloadLength"` // payload length
	Payload       []byte `json:"payload"`       // payload
}

// RoomMessage used only with a room's Send channel
type RoomMessage struct {
	Sender *Conn
	Data   []byte
}

// BytesToMessage returns a Message type from bytes
func BytesToMessage(data []byte) *Message {
	msg := new(Message)
	json.Unmarshal(data, msg)
	return msg
}

// Bytes returns bytes from a Message type
func (msg *Message) Bytes() []byte {
	b, _ := json.Marshal(msg)
	return b
}

// ConstructMessage constructs and returns a new Message type
func ConstructMessage(room, event, dst, src string, payload []byte) *Message {
	return &Message{
		RoomLength:    len(room),
		Room:          room,
		EventLength:   len(event),
		Event:         event,
		DstLength:     len(dst),
		Dst:           dst,
		SrcLength:     len(src),
		Src:           src,
		PayloadLength: len(payload),
		Payload:       payload,
	}
}

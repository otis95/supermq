// write by lzq
package message

import (
	"fmt"
)

type RouteMessage struct {
	header

	info []byte
}

var _ Message = (*RouteMessage)(nil)

func NewRouteMessage() *RouteMessage {
	msg := &RouteMessage{}
	msg.SetType(CONNECT_ROUTE)

	return msg
}

func (this RouteMessage) String() string {
	return fmt.Sprintf("%s, Info=%v", this.header, this.info)
}

// SetTopic sets the the topic name that identifies the information channel to which
// payload data is published. An error is returned if ValidTopic() is falbase.
func (this *RouteMessage) SetInfo(v []byte) error {

	this.info = v

	return nil
}

func (this *RouteMessage) GetInfo() []byte {
	return this.info
}

func (this *RouteMessage) Len() int {

	ml := this.msglen()

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0
	}

	return this.header.msglen() + ml
}

func (this *RouteMessage) Decode(src []byte) (int, error) {
	total := 0

	hn, _ := this.header.decode(src[total:])
	total += hn

	this.info = src[hn:]
	total += len(this.info)

	return total, nil
}

func (this *RouteMessage) Encode(dst []byte) (int, error) {

	if len(this.info) == 0 {
		return 0, fmt.Errorf("route/Encode: info  is empty.")
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0, err
	}

	hl := this.header.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("route/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
	}

	total := 0

	n, err := this.header.encode(dst[total:])
	total += n
	if err != nil {
		return total, err
	}

	copy(dst[total:], this.info)
	total += len(this.info)

	return total, nil
}

func (this *RouteMessage) msglen() int {
	total := len(this.info)

	return total
}

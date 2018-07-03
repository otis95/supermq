package topics

import (
	"fmt"
	"testing"
)

func Test_Topic(t *testing.T) {
	mem := NewMemProvider()
	mem.Subscribe([]byte("appliction/+/node/#"), 0x01, 1)
	mem.Subscribe([]byte("appliction/+/node111111"), 0x01, 1)
	mem.Subscribe([]byte("appliction/+/level"), 0x01, 1)
	mem.Subscribe([]byte("appliction/123/node/#"), 0x01, 1)
	mem.Subscribe([]byte("123/321/node/#"), 0x01, 1)
	fmt.Println(mem.sroot.stopic())
}

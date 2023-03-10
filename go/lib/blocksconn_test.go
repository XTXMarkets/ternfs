package lib

import (
	"testing"
	"time"
	"xtx/eggsfs/msgs"

	"github.com/stretchr/testify/assert"
)

func TestBlocksConn(t *testing.T) {
	factory := blocksConnFactory{
		cached: make(map[msgs.BlockServiceId]*cachedBlocksConns),
	}
	bs := msgs.BlockServiceId(0)
	t0 := time.Now()
	conn, err := factory.get(bs, t0, func() (any, error) {
		return "conn1", nil
	})
	assert.NoError(t, err)
	factory.put(bs, t0.Add(blocksConnsTimeout/10), conn)
	conn, err = factory.get(bs, t0.Add(2*blocksConnsTimeout/10), func() (any, error) {
		return "conn2", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "conn1", conn)
	factory.put(bs, t0.Add(3*blocksConnsTimeout/10), conn)
	conn, err = factory.get(bs, t0.Add(blocksConnsTimeout*2), func() (any, error) {
		return "conn3", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "conn3", conn)
}

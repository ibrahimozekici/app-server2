package eventlog

import (
	"bytes"
	"context"
	"log"
	"testing"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"

	pb "github.com/ibrahimozekici/lora-api/go/v3/as/integration"
	"github.com/ibrahimozekici/app-server2/internal/storage"
	"github.com/ibrahimozekici/app-server2/internal/test"
	"github.com/brocaar/lorawan"
)

func TestEventLog(t *testing.T) {
	assert := require.New(t)

	conf := test.GetConfig()
	assert.NoError(storage.Setup(conf))

	storage.RedisClient().FlushAll()

	upEvent := pb.UplinkEvent{
		Data: []byte{0x01, 0x02, 0x03, 0x03},
	}

	t.Run("GetEventLogForDevice", func(t *testing.T) {
		devEUI := lorawan.EUI64{1, 2, 3, 4, 5, 6, 7, 8}
		logChannel := make(chan EventLog, 1)
		ctx := context.Background()
		cctx, cancel := context.WithCancel(ctx)
		defer cancel()

		go func() {
			if err := GetEventLogForDevice(cctx, devEUI, logChannel); err != nil {
				log.Fatal(err)
			}
		}()

		// some time to subscribe
		time.Sleep(time.Millisecond * 100)

		t.Run("LogEventForDevice", func(t *testing.T) {
			assert := require.New(t)
			assert.NoError(LogEventForDevice(devEUI, Uplink, &upEvent))

			el := <-logChannel

			var pl pb.UplinkEvent
			um := &jsonpb.Unmarshaler{
				AllowUnknownFields: true,
			}
			assert.NoError(um.Unmarshal(bytes.NewReader(el.Payload), &pl))

			assert.Equal(Uplink, el.Type)
			assert.True(proto.Equal(&upEvent, &pl))
		})
	})
}

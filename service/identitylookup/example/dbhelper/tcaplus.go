package dbhelper

import (
	tcaplus "github.com/tencentyun/tcaplusdb-go-sdk/pb"
	"google.golang.org/protobuf/proto"
)

type TcaplusConfig struct {
	AppId     uint64
	ZoneId    uint32
	Addr      string
	Signature string
}

func InitPBClient(cfg *TcaplusConfig, tables []proto.Message) (*tcaplus.PBClient, error) {
	cli := tcaplus.NewPBClient()

	m := make([]string, len(tables))
	for i, tb := range tables {
		n := tb.ProtoReflect().Descriptor().Name()
		m[i] = string(n)
	}

	err := cli.Dial(cfg.AppId, []uint32{cfg.ZoneId}, cfg.Addr,
		cfg.Signature, 10, map[uint32][]string{
			cfg.ZoneId: m,
		})
	if err != nil {
		return nil, err
	}
	return cli, nil
}

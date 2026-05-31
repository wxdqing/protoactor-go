package main

import (
	"fmt"

	"gitee.com/wxdqing/identitylookup/example/dbhelper"
	table "gitee.com/wxdqing/identitylookup/example/dbproto/tcaplus"

	"gitee.com/wxdqing/identitylookup/tcaplus_storage"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func main() {
	cfg := &dbhelper.TcaplusConfig{
		AppId:     207,
		ZoneId:    2,
		Addr:      "tcp://21.6.0.48:9999",
		Signature: "72E858546B3CCBC6@",
	}
	tables := []proto.Message{
		&table.ActorRouter{},
	}

	client, err := dbhelper.InitPBClient(cfg, tables)
	if err != nil {
		panic(err)
	}

	tb := tcaplus_storage.NewTcaplusTable(client, cfg.ZoneId, "ActorRouter", &table.ActorRouter{}, []string{"Kind", "Identity"})

	identity := uuid.NewString()
	data, version, err := tb.InsertRecord([]string{"test", identity}, &table.ActorRouter{})
	if err != nil {
		panic(err)
	}

	fmt.Printf("insert success %v %v\n", data, version)

	data, version, err = tb.GetRecord([]string{"test", identity})
	if err != nil {
		panic(err)
	}
	fmt.Printf("get success %v %v\n", data, version)

	data.(*table.ActorRouter).Member = "test"
	data, version, err = tb.UpdateRecord([]string{"test", identity}, data, version)
	if err != nil {
		panic(err)
	}
	fmt.Printf("update1 success %v %v\n", data, version)

	data.(*table.ActorRouter).Address = "test1"
	data, version, err = tb.UpdateRecord([]string{"test", identity}, data, version)
	if err != nil {
		panic(err)
	}
	fmt.Printf("update2 success %v %v\n", data, version)

	router := &table.ActorRouter{}
	desc := router.ProtoReflect().Descriptor()
	fields := desc.Fields()
	for k := 0; k < fields.Len(); k++ {
		fieldDesc := fields.Get(k)
		fmt.Printf("field name %v\n", fieldDesc.Name())
	}

	fmt.Printf("protoreflect field name %v\n", protoreflect.Name("lockTime"))

	data, version, err = tb.GetRecord([]string{"test", identity})
	if err != nil {
		panic(err)
	}
	fmt.Printf("get success %v %v\n", data, version)
}

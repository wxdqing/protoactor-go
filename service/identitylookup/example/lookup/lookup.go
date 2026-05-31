package main

import (
	"fmt"
	"time"

	"gitee.com/wxdqing/identitylookup"
	"gitee.com/wxdqing/identitylookup/example/dbhelper"
	table "gitee.com/wxdqing/identitylookup/example/dbproto/tcaplus"
	"gitee.com/wxdqing/identitylookup/example/lookup/grain"
	"gitee.com/wxdqing/identitylookup/redis_storage"
	tcaplus "gitee.com/wxdqing/identitylookup/tcaplus_storage"

	grain_proto "gitee.com/wxdqing/identitylookup/example/lookup/grain/proto"

	console "github.com/asynkron/goconsole"
	"github.com/asynkron/protoactor-go/actor"
	"github.com/asynkron/protoactor-go/cluster"
	"github.com/asynkron/protoactor-go/cluster/clusterproviders/etcd"
	"github.com/asynkron/protoactor-go/remote"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"
)

func main() {
	c := startNode()

	cli := grain_proto.GetHelloGrainClient(c, "world")
	for i := 0; i < 10; i++ {
		go func() {
			res, err := cli.SayHello(&grain_proto.HelloRequest{Name: uuid.NewString()})
			fmt.Printf("say hello res %v %v\n", res, err)
		}()
	}

	time.Sleep(time.Second * 5)
	cli1 := grain_proto.GetManualGrainClient(c, "world")
	_, err := cli1.Ping(&grain_proto.PingRequest{Name: uuid.NewString()})
	if err != nil {
		fmt.Printf("say hello1 err %v\n", err)
	}

	activator := c.Config.IdentityLookup.(*identitylookup.StorageIdentityLookup)
	pid := activator.Activate(cluster.NewClusterIdentity("world", "Manual"))

	for i := 0; i < 10; i++ {
		go func() {
			res, err := cli1.Ping(&grain_proto.PingRequest{Name: uuid.NewString()})
			fmt.Printf("say hello1 res %v %v\n", res, err)
		}()
	}

	time.Sleep(time.Second * 5)
	c.ActorSystem.Root.Poison(pid)

	fmt.Println()
	console.ReadLine()
	c.Shutdown(true)
}

func startNode() *cluster.Cluster {
	system := actor.NewActorSystem()
	// system := actor.NewActorSystem(identitylookup.NewSystemIDOption("game-0", 1234))
	etcdCfg := clientv3.Config{
		Endpoints:   []string{"9.135.91.219:2379"},
		DialTimeout: 3 * time.Second,
	}
	provider, err := etcd.NewWithConfig("rmini-identitylookup-test1", etcdCfg)
	if err != nil {
		panic("failed to connect to etcd: err:%s" + err.Error())
	}

	//lookup := disthash.New()
	//lookup := identitylookup.NewStorageIdentityLookup(createRedisStorage(), []string{"Manual"})
	lookup := identitylookup.NewStorageIdentityLookup(createTcaplusStorage(), []string{"Manual"})
	helloKind := grain_proto.NewHelloKind(func() grain_proto.Hello {
		return &grain.HelloGrain{}
	}, 0)
	manualKind := grain_proto.NewManualKind(func() grain_proto.Manual {
		return &grain.ManualGrain{}
	}, 0)
	config := remote.Configure("localhost", 0)

	clusterConfig := cluster.Configure("rmini-identitylookup-test", provider, lookup, config, cluster.WithKinds(helloKind, manualKind))
	c := cluster.New(system, clusterConfig)
	c.StartMember()

	fmt.Printf("start member success\n")
	return c
}

func createTcaplusStorage() identitylookup.IdentityStorage {
	cfg := &dbhelper.TcaplusConfig{
		AppId:     207,
		ZoneId:    3,
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

	tableDesc := &tcaplus.RouterTableDesc{
		TableName:          "ActorRouter",
		MessageType:        &table.ActorRouter{},
		IdentityFields:     []string{"Kind", "Identity"},
		PidFields:          []string{"Address", "ActorID"},
		MemberIdField:      "Member",
		LockTimeField:      "LockTime",
		TerminateTimeField: "TerminateTime",
		ActivateTimeField:  "ActiveTime",
	}
	dataAccess := tcaplus.NewIdentityDataAccess(client, cfg.ZoneId, tableDesc)
	identityStorage := identitylookup.NewIdentityStorage(dataAccess)
	return identityStorage
}

/*

Host = "9.135.111.208:6380"
Password = "ssAB333%%%@S"
Index = 0
*/

func createRedisStorage() identitylookup.IdentityStorage {
	redisCli := dbhelper.NewRedisClient()
	dataAccess := redis_storage.NewIdentityDataAccess(redisCli, "rmini-identitylookup-test1")
	identityStorage := identitylookup.NewIdentityStorage(dataAccess)
	//identityStorage := redis_storage.NewIdentityStorage(redisCli, "rmini-identitylookup-test1")
	return identityStorage
}

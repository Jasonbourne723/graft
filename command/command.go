package command

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"raft/raft"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

var rootCommand = &cobra.Command{
	Use:   "raft",
	Short: "raft is a simple raft implementation",
	Long:  "raft is a simple raft implementation",
	Run: func(cmd *cobra.Command, args []string) {

		server, _ := cmd.Flags().GetString("server")
		others, _ := cmd.Flags().GetString("others")
		serverId, _ := cmd.Flags().GetInt32("serverId")

		listenerer, err := net.Listen("tcp", server)
		if err != nil {
			fmt.Println("Failed to listen: ", err)
			return
		}

		nodeEndPoints := strings.Split(others, ",")
		nodes := make([]*raft.Node, len(nodeEndPoints))
		for i, endPoint := range nodeEndPoints {
			nodes[i] = &raft.Node{EndPoint: endPoint}
		}

		grpcServer := grpc.NewServer()
		raftServer := raft.NewServer(raft.WithOthers(nodes), raft.WithId(serverId))
		raftServer.Start()

		raft.RegisterRaftServer(grpcServer, raftServer)

		defer func() {
			grpcServer.Stop()
			listenerer.Close()
		}()

		if err = grpcServer.Serve(listenerer); err != nil {
			fmt.Println("Failed to serve: ", err)
			return
		}

		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)

		<-signalChan
	},
}

func Execute() {

	rootCommand.PersistentFlags().String("server", "127.0.0.1:8080", "服务器地址和端口")
	rootCommand.PersistentFlags().Int32("serverId", 0, "节点ID")
	rootCommand.PersistentFlags().String("others", "", "其他节点地址, 逗号分隔")
	if err := rootCommand.Execute(); err != nil {
		log.Fatal("报错" + err.Error())
	}
}

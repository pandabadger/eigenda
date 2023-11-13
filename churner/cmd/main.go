package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	pb "github.com/Layr-Labs/eigenda/api/grpc/churner"
	"github.com/Layr-Labs/eigenda/churner"
	"github.com/Layr-Labs/eigenda/churner/flags"
	"github.com/Layr-Labs/eigenda/common/geth"
	"github.com/Layr-Labs/eigenda/common/healthcheck"
	"github.com/Layr-Labs/eigenda/common/logging"
	"github.com/Layr-Labs/eigenda/core/eth"
	coreeth "github.com/Layr-Labs/eigenda/core/eth"
	"github.com/Layr-Labs/eigenda/core/thegraph"
	"github.com/shurcooL/graphql"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	Version   = ""
	GitCommit = ""
	GitDate   = ""
)

func main() {
	app := cli.NewApp()
	app.Version = fmt.Sprintf("%s-%s-%s", Version, GitCommit, GitDate)
	app.Name = "churner"
	app.Usage = "EigenDA Churner"
	app.Description = "Service manages contract registrations, facilitates operator removal, and gathers deregistration information from operators."
	app.Flags = flags.Flags
	app.Action = run
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("application failed: %v", err)
	}

	select {}
}

func run(ctx *cli.Context) error {
	log.Println("Initializing churner")
	hostname := "0.0.0.0"
	port := ctx.String(flags.GrpcPortFlag.Name)
	addr := fmt.Sprintf("%s:%s", hostname, port)
	log.Println("Starting churner server at", addr)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalln("could not start tcp listener", err)
	}

	opt := grpc.MaxRecvMsgSize(1024 * 1024 * 300)
	gs := grpc.NewServer(
		opt,
		grpc.ChainUnaryInterceptor(),
	)

	config := churner.NewConfig(ctx)
	logger, err := logging.GetLogger(config.LoggerConfig)
	if err != nil {
		return err
	}

	log.Println("Starting geth client")
	gethClient, err := geth.NewClient(config.EthClientConfig, logger)
	if err != nil {
		log.Fatalln("could not start tcp listener", err)
	}

	tx, err := eth.NewTransactor(logger, gethClient, config.BLSOperatorStateRetrieverAddr, config.EigenDAServiceManagerAddr)
	if err != nil {
		log.Fatalln("could create new transactor", err)
	}

	cs := coreeth.NewChainState(tx, gethClient)

	querier := graphql.NewClient(config.GraphUrl, nil)
	indexer := thegraph.NewIndexedChainState(cs, querier, logger)

	cn, err := churner.NewChurner(config, indexer, tx, logger)
	if err != nil {
		log.Fatalln("cannot create churner", err)
	}

	churnerServer := churner.NewServer(config, cn, logger)
	if err = churnerServer.Start(context.Background()); err != nil {
		log.Fatalln("failed to start churner server", err)
	}

	// Register reflection service on gRPC server
	// This makes "grpcurl -plaintext localhost:9000 list" command work
	reflection.Register(gs)

	pb.RegisterChurnerServer(gs, churnerServer)

	// Register Server for Health Checks
	healthcheck.RegisterHealthServer(gs)

	log.Printf("churner server listening at %s", addr)
	return gs.Serve(listener)
}

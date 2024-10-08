package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
)

type ServiceContext struct {
	nc        *nats.Conn
	ddb       *dynamodb.Client
	table     string
	serviceID string
}

func startService(nc *nats.Conn, ddb *dynamodb.Client, table string) error {
	svc, err := micro.AddService(nc, micro.Config{
		Name:        "customer",
		Version:     "0.0.1",
		Description: "customer service",
	})
	if err != nil {
		return err
	}

	handlerCtx := &ServiceContext{nc: nc, ddb: ddb, table: table, serviceID: svc.Info().ID}

	root := svc.AddGroup("customer")
	root.AddEndpoint("balance", micro.HandlerFunc(handlerCtx.GetBalance),
		micro.WithEndpointMetadata(map[string]string{
			"description": "Create or update a customer",
			"format":      "application/json",
		}))

	return nil
}

type BalanceRequest struct {
	CustomerID string `json:"client_id"`
}

type BalanceResponse struct {
	Balance int `json:"balance"`
}

func (sc *ServiceContext) GetBalance(req micro.Request) {
	ctx := context.TODO()
	log.Printf("service_id: %s", sc.serviceID)

	// Decode the request
	var balanceReq BalanceRequest
	err := json.Unmarshal([]byte(req.Data()), &balanceReq)
	if err != nil {
		req.Error("403", "BAD_REQUEST", []byte(err.Error()))
		return
	}

	br, err := getBalance(ctx, sc.ddb, sc.table, balanceReq.CustomerID)
	if err != nil {
		req.Error("500", "INTERNAL_ERROR  - retrieve bal", []byte("client_balance  error"))
		return
	}

	bal := BalanceResponse{Balance: br}

	// Encode the response
	resp, err := json.Marshal(bal)
	if err != nil {
		req.Error("500", "INTERNAL_ERROR - encode json", []byte(err.Error()))
		return
	}

	// Publish the response for Data Platform to read
	err = sc.nc.Publish(fmt.Sprintf("customer.balance.%s", balanceReq.CustomerID), resp)
	if err != nil {
		req.Error("500", "INTERNAL_ERROR - publish", []byte(err.Error()))
		return
	}

	req.Respond(resp)
}

package app

import (
	"context"
	"fmt"

	"github.com/reubenmiller/go-c8y/pkg/c8y"
)

type TestStruct struct {
	tenantId  string
	ctx       context.Context
	c8yClient *c8y.Client
}

func (c *TestStruct) CreateDevice() {
	mo, resp, err := c.c8yClient.Inventory.CreateDevice(c.ctx, "test")
	if err != nil {
		fmt.Println("Error: " + err.Error())
	}
	fmt.Println("Server response" + resp.String())
	fmt.Println("MO ID: " + mo.ID)
}

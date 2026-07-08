package main

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/tanselxy/singbox/internal/config"
	"github.com/tanselxy/singbox/internal/v2rayapi"
)

// runStatsDump dials the v2ray_api and prints all counters (no reset). Hidden
// debug command used to verify per-user traffic accounting.
func runStatsDump(ctx context.Context) error {
	dctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dctx, config.V2RayAPIAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("dial %s: %w", config.V2RayAPIAddr, err)
	}
	defer conn.Close()

	client := v2rayapi.NewStatsServiceClient(conn)
	resp, err := client.QueryStats(ctx, &v2rayapi.QueryStatsRequest{Patterns: []string{""}})
	if err != nil {
		return fmt.Errorf("QueryStats: %w", err)
	}
	fmt.Printf("v2ray_api returned %d counters:\n", len(resp.GetStat()))
	for _, s := range resp.GetStat() {
		fmt.Printf("  %s = %d\n", s.GetName(), s.GetValue())
	}
	return nil
}

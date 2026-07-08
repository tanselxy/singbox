package main

import (
	"context"
	"flag"
	"fmt"
)

// runUninstall removes sing-box, the panel and all generated artifacts. Implemented in P4.
func runUninstall(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "跳过确认")
	if err := fs.Parse(args); err != nil {
		return err
	}

	_ = ctx
	_ = yes
	return fmt.Errorf("uninstall 尚未实现（P4）")
}

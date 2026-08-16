package main

import (
	"time"

	"github.com/bylexus/linkomat/shared"
	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP server (default)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := LoadConfig()
			if err != nil {
				return err
			}
			meta, err := OpenMetaDB(cfg)
			if err != nil {
				return err
			}
			defer meta.Close()
			userDBs := NewUserDBs(cfg)
			defer userDBs.Close()
			srv := NewServer(cfg, meta, userDBs)

			go runRateLimitCleanup(srv.limiter)

			cmd.Println("linkomat-server", shared.Version, "listening on", cfg.Listen)
			return srv.ListenAndServe()
		},
	}
}

func runRateLimitCleanup(l *loginLimiter) {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for range t.C {
		l.cleanup()
	}
}
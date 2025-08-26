package main

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"log/slog"
	"mbvlabs/andurel/layout/elements/config"
	"mbvlabs/andurel/layout/elements/database"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

func main() {
	ctx := context.Background()
	ctx, cancel := context.WithTimeoutCause(
		ctx,
		5*time.Minute,
		errors.New("migration timeout of 5 minutes reached"),
	)
	defer cancel()

	// cfg := config.NewConfig()

	gooseLock, err := lock.NewPostgresSessionLocker()
	if err != nil {
		panic(err)
	}

	fsys, err := fs.Sub(database.Migrations, "migrations")
	if err != nil {
		panic(err)
	}

	pool, err := database.CreatePooledConnection(ctx, config.DB.GetDatabaseURL())
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	db := stdlib.OpenDBFromPool(pool)

	gooseProvider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		fsys,
		goose.WithVerbose(true),
		goose.WithSessionLocker(gooseLock),
	)
	if err != nil {
		panic(err)
	}

	ops := flag.String("cmd", "", "")
	version := flag.String("version", "", "")
	flag.Parse()

	if *version != "" {
		v, err := strconv.Atoi(*version)
		if err != nil {
			panic(err)
		}

		switch *ops {
		case "up":
			_, err = gooseProvider.UpTo(ctx, int64(v))
			if err != nil {
				panic(err)
			}
		case "down":
			_, err = gooseProvider.DownTo(ctx, int64(v))
			if err != nil {
				panic(err)
			}
		}
	}

	if *version == "" {
		switch *ops {
		case "up":
			_, err = gooseProvider.Up(ctx)
			if err != nil {
				panic(err)
			}
		case "down":
			_, err = gooseProvider.Down(ctx)
			if err != nil {
				panic(err)
			}
		case "upbyone":
			_, err = gooseProvider.UpByOne(ctx)
			if err != nil {
				panic(err)
			}
		case "reset":
			_, err = gooseProvider.DownTo(ctx, 0)
			if err != nil {
				panic(err)
			}
			_, err = gooseProvider.Up(ctx)
			if err != nil {
				panic(err)
			}
		case "status":
			statuses, err := gooseProvider.Status(ctx)
			if err != nil {
				panic(err)
			}

			for _, status := range statuses {
				slog.InfoContext(
					ctx,
					"database status",
					"version",
					status.Source.Version,
					"file_name",
					status.Source.Path,
					"state",
					status.State,
					"applied_at",
					status.AppliedAt,
				)
			}
		}
	}
}

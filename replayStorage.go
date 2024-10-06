package main

import (
	"context"
	"errors"
	"log"

	"github.com/DataDog/zstd"
	"github.com/jackc/pgx/v4"
)

var errReplayNotFound = errors.New("replay not found")

func getReplayFromStorage(ctx context.Context, gid int) ([]byte, error) {
	var compressedReplay []byte
	err := dbpool.QueryRow(ctx, `select replay from games where id = $1`, gid).Scan(&compressedReplay)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("Error fetching replay from database: %s", err.Error())
	}
	if len(compressedReplay) > 0 {
		return zstd.Decompress(nil, compressedReplay)
	}
	return nil, errReplayNotFound
}

func checkReplayExistsInStorage(ctx context.Context, gid int) bool {
	var replayPresent int
	err := dbpool.QueryRow(ctx, `select count(replay) from games where id = $1 and replay is not null`, gid).Scan(&replayPresent)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Printf("Error fetching replay from database: %s", err.Error())
		return false
	}
	return replayPresent == 1
}

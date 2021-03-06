// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"

	"flag"
	"log"

	"github.com/google/trillian-examples/sumdbaudit/audit"
	_ "github.com/mattn/go-sqlite3"
)

var (
	height = flag.Int("h", 8, "tile height")
	vkey   = flag.String("k", "sum.golang.org+033de0ae+Ac4zctda0e5eza+HJyk9SxEdh+s3Ux18htTTAD8OuAn8", "key")
	db     = flag.String("db", "./sum.db", "database file location (will be created if it doesn't exist)")
	extraV = flag.Bool("x", false, "performs additional checks on each tile hashes")
)

// Clones the leaves of the SumDB into the local database and verifies the result.
// This does not perform any checks on the leaf data to look for inconsistent claims.
// If this returns successfully, it means that all leaf data in the DB matches that
// contained in the SumDB.
func main() {
	ctx := context.Background()

	log.SetPrefix("clone: ")
	log.SetFlags(0)
	flag.Parse()

	db, err := audit.NewDatabase(*db)
	if err != nil {
		log.Fatalf("failed to open DB: %v", err)
	}
	err = db.Init()
	if err != nil {
		log.Fatalf("failed to init DB: %v", err)
	}

	sumDB := audit.NewSumDB(*height, *vkey)
	checkpoint, err := sumDB.LatestCheckpoint()
	if err != nil {
		log.Fatalf("failed to get latest checkpoint: %s", err)
	}

	log.Printf("Got SumDB checkpoint for %d entries. Downloading...", checkpoint.N)
	s := audit.NewService(db, sumDB, *height)
	if err := s.CloneLeafTiles(ctx, checkpoint); err != nil {
		log.Fatalf("failed to update leaves: %v", err)
	}
	log.Printf("Updated leaves to latest checkpoint (tree size %d). Calculating hashes...", checkpoint.N)

	if err := s.HashTiles(ctx, checkpoint); err != nil {
		log.Fatalf("HashTiles: %v", err)
	}
	log.Printf("Hashes updated successfully. Checking root hash...")
	if err := s.CheckRootHash(ctx, checkpoint); err != nil {
		log.Fatalf("CheckRootHash: %v", err)
	}
	log.Printf("Cloned successfully. Tree size is %d, hash is %x (%s). Processing data...", checkpoint.N, checkpoint.Hash[:], checkpoint.Hash)

	if err := s.ProcessMetadata(ctx, checkpoint); err != nil {
		log.Fatalf("ProcessMetadata: %v", err)
	}
	log.Printf("Leaf data processed.")
	if *extraV {
		log.Printf("Performing extra validation on tiles...")
		if err := s.VerifyTiles(ctx, checkpoint); err != nil {
			log.Fatalf("VerifyTiles: %v", err)
		}
		log.Printf("Tile verificaton passed")
	}
}

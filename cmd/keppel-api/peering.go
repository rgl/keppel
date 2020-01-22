/******************************************************************************
*
*  Copyright 2020 SAP SE
*
*  Licensed under the Apache License, Version 2.0 (the "License");
*  you may not use this file except in compliance with the License.
*  You may obtain a copy of the License at
*
*      http://www.apache.org/licenses/LICENSE-2.0
*
*  Unless required by applicable law or agreed to in writing, software
*  distributed under the License is distributed on an "AS IS" BASIS,
*  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*  See the License for the specific language governing permissions and
*  limitations under the License.
*
******************************************************************************/

package main

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"time"

	"github.com/sapcc/go-bits/logg"
	"github.com/sapcc/keppel/internal/keppel"
	"github.com/sapcc/keppel/internal/tasks"
)

func runPeering(ctx context.Context, cfg keppel.Configuration, db *keppel.DB) {
	var peerHostNames []string
	for _, hostName := range strings.Split(os.Getenv("KEPPEL_PEERS"), ",") {
		hostName = strings.TrimSpace(hostName)
		if hostName != "" {
			peerHostNames = append(peerHostNames, hostName)
		}
	}

	if len(peerHostNames) == 0 {
		//nothing to do
		return
	}

	//add missing entries to `peers` table
	for _, peerHostName := range peerHostNames {
		_, err := db.Exec(
			`INSERT INTO peers (hostname) VALUES ($1) ON CONFLICT DO NOTHING`,
			peerHostName,
		)
		must(err)
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				err := tryIssueNewPasswordForPeer(cfg, db)
				if err != nil {
					logg.Error("cannot issue new peer password: " + err.Error())
				}
			}
		}
	}()
}

//WARNING: This must be run in a transaction, or else `FOR UPDATE SKIP LOCKED`
//will not work as expected.
const getNextPeerQuery = `
	SELECT * FROM peers
	 WHERE last_peered_at < $1 OR last_peered_at IS NULL
	 ORDER BY COALESCE(last_peered_at, TO_TIMESTAMP(-1)) ASC LIMIT 1
	   FOR UPDATE SKIP LOCKED
`

func tryIssueNewPasswordForPeer(cfg keppel.Configuration, db *keppel.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer keppel.RollbackUnlessCommitted(tx)

	//select next peer that needs a new password, if any
	var peer keppel.Peer
	err = tx.SelectOne(&peer, getNextPeerQuery, time.Now().Add(-10*time.Minute))
	if err == sql.ErrNoRows {
		//nothing to do
		tx.Rollback() //avoid the log line generated by keppel.RollbackUnlessCommitted()
		return nil
	}
	if err != nil {
		return err
	}

	//issue password (this will also commit the transaction)
	return tasks.IssueNewPasswordForPeer(cfg, db, tx, peer)
}

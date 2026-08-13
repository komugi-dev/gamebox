package clientutil

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/beevik/guid"
	"github.com/komugi-dev/gamebox/client"
	log "github.com/sirupsen/logrus"
)

func SetupPlayer(ctx context.Context, gbURL *url.URL, tablePrefix string, pName string) (*client.Player, error) {
	var joinedTable client.Table

	cli := http.Client{
		Timeout: 10 * time.Second,
	}

	// connect to gamebox
	ss := client.CreateSession(&cli, *gbURL)

	// create a player
	pl := client.CreatePlayer(pName, ss)

	// get tables
	existingTables, err := ss.Tables(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot get tables; %w", err)
	}
	log.Infof("found %v tables", len(existingTables))

	// as there are 2 players per table,
	// if joining an existing table works, start playing...
	for _, t := range existingTables {
		log.Infof("joining table %+v", t)
		err = pl.Join(ctx, t)
		if err != nil {
			log.Warningf("cannot join table %v (%v)", t.Summary.TableGUID, err)
			continue
		}
		joinedTable = t
		err = joinedTable.Start(ctx)
		if err != nil {
			log.Warningf("cannot start table; %v", err)
			continue
		}
		log.Infof("joined and started table %+v", joinedTable.Summary)
		break
	}
	// ...otherwise, create a new table and wait for another player
	if joinedTable.Summary.TableGUID == "" {
		joinedTable, err = ss.Table(ctx, "tic-tac-toe-"+guid.NewString())
		if err != nil {
			return nil, fmt.Errorf("cannot create new table; %w", err)
		}
		log.Infof("created new table %+v", joinedTable.Summary)

		log.Infof("joining table %+v", joinedTable)
		err = pl.Join(ctx, joinedTable)
		if err != nil {
			return nil, fmt.Errorf("cannot join table %v (%v)", joinedTable.Summary.TableGUID, err)
		}
	}

	return pl, nil
}

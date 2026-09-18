package logic

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func parsePayload(t *testing.T, raw json.RawMessage) MsgPlayerPayload {
	var p MsgPlayerPayload
	err := json.Unmarshal(raw, &p)
	if err != nil {
		t.Fatalf("error decoding payload: %v", err)
	}
	return p
}

func TestGetLosers(t *testing.T) {
	tests := []struct {
		name  string
		moves map[string]choice
		want  []string
	}{
		{
			name:  "R->S",
			moves: map[string]choice{"p1": Rock, "p2": Scissor},
			want:  []string{"p2"},
		},
		{
			name:  "P->R",
			moves: map[string]choice{"p1": Rock, "p2": Paper},
			want:  []string{"p1"},
		},
		{
			name:  "no losers",
			moves: map[string]choice{"p1": Rock, "p2": Rock, "p3": Rock},
			want:  []string{},
		},
		{
			name:  "all losers",
			moves: map[string]choice{"p1": Rock, "p2": Paper, "p3": Scissor},
			want:  []string{"p1", "p2", "p3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getLosers(tt.moves)

			slices.Sort(got)
			slices.Sort(tt.want)

			if !slices.Equal(got, tt.want) {
				t.Errorf("getLosers() = %v, atteso %v", got, tt.want)
			}
		})
	}
}

func TestGameFlow_StandardMatch(t *testing.T) {
	rps := CreateRPSLogic()

	// lobby
	_, err := rps.AddPlayer("secret_1", "Alice")
	assert.Nil(t, err)

	rps.AddPlayer("secret_2", "Bob")
	rps.AddPlayer("secret_3", "Charlie")
	assert.Equal(t, 3, rps.Info().Players)

	// start
	gu, err := rps.Start()
	assert.Nil(t, err)
	assert.Equal(t, 3, len(gu.NextPlayers))

	// play
	rockMove := json.RawMessage(`{"choice": 0}`)
	scissorMove := json.RawMessage(`{"choice": 2}`)

	// Alice
	gu, _ = rps.Play("secret_1", rockMove)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(gu.Status)) // no updates for the intermediate moves

	// Bob
	gu, err = rps.Play("secret_2", scissorMove)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(gu.Status))

	// Charlie - closes the turn
	gu, err = rps.Play("secret_3", scissorMove)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(gu.Status))

	// test the turn
	assert.True(t, gu.IsGameOver)
	assert.Equal(t, 1, len(gu.NextPlayers))
	assert.Equal(t, "secret_1", gu.NextPlayers[0])

	// test Alice's payload
	alicePayload := parsePayload(t, gu.Status["secret_1"])
	assert.Equal(t, "secret_1", alicePayload.Turn.Self)

	// test Bob's payload
	bobPayload := parsePayload(t, gu.Status["secret_2"])
	assert.Equal(t, "", bobPayload.Turn.Self)
	assert.True(t, slices.Contains(bobPayload.Turn.Survivors, "Alice"))
}

func TestRemovePlayer_AvoidsDeadlock(t *testing.T) {
	rps := CreateRPSLogic()
	rps.AddPlayer("secret_1", "Alice")
	rps.AddPlayer("secret_2", "Bob")
	rps.AddPlayer("secret_3", "Charlie")
	gu, err := rps.Start()
	assert.Nil(t, err)
	assert.Equal(t, 3, len(gu.NextPlayers))

	// Alice plays
	rockMove := json.RawMessage(`{"choice": 0}`)
	gu, err = rps.Play("secret_1", rockMove)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(gu.NextPlayers))
	assert.Equal(t, 0, len(gu.Status))

	// Bob and Charlie don't move, but Bob leaves
	gu, err = rps.RemovePlayer("secret_2")
	assert.Nil(t, err)
	assert.Equal(t, 0, len(gu.NextPlayers))
	assert.Equal(t, 2, len(gu.Status))

	// verifies payload contains the leaver
	alicePayload := parsePayload(t, gu.Status["secret_1"])
	assert.Equal(t, "Bob", alicePayload.Leaver)

	// now Charlies leaves
	gu, err = rps.RemovePlayer("secret_3")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(gu.Status))
	assert.Equal(t, 1, len(gu.NextPlayers)) // Alice wins, it should be in the NextPlayers
	assert.Equal(t, gu.NextPlayers[0], "secret_1")

	// only Alice is in the game, so she wins
	assert.True(t, gu.IsGameOver)

	charliePayload := parsePayload(t, gu.Status["secret_1"])
	assert.Equal(t, "Charlie", charliePayload.Leaver)
}

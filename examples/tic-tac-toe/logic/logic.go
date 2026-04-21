package logic

import "github.com/komugi-dev/gamebox"

type T3Logic struct{}

func CreateT3() gamebox.GameRules {
	return &T3Logic{}
}

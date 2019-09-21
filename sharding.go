package main

import (
	"github.com/Seklfreak/Robyul2/shardmanager"
)

func newSharder(token string) (*shardmanager.Manager, error) {
	manager := shardmanager.New(token)                  // TODO
	manager.Name = "Robyul"                             // TODO
	manager.LogChannel = "271740860180201473"           // TODO
	manager.StatusMessageChannel = "271740860180201473" // TODO

	recommended, err := manager.GetRecommendedCount()
	if err != nil {
		return nil, err
	}
	if recommended < 2 {
		manager.SetNumShards(5)
	}

	err = manager.Start()
	if err != nil {
		return nil, err
	}

	return manager, nil
}

// func createSharding(token string) (*discordgo.Session, error) {
// 	gateway, err := discordgo.New(token)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	s, err := gateway.GatewayBot()
// 	if err != nil {
// 		return nil, err
// 	}
// 	s.Shards = 1
// 	// TODO
//
// 	fakeSession := &discordgo.Session{}
//
// 	sessions := make([]*discordgo.Session, s.Shards)
//
// 	for i := 0; i < s.Shards; i++ {
// 		session, err := discordgo.New(token)
// 		if err != nil {
// 			return nil, err
// 		}
// 		session.ShardCount = s.Shards
// 		session.ShardID = i
// 		session.AddHandler(func(_ *discordgo.Session, i interface{}) {
// 			fakeSession.
// 		})
//
// 		sessions[i] = session
// 	}
//
// 	for i := 0; i < len(sessions); i++ {
// 		err = sessions[i].Open()
// 	}
//
// 	return fakeSession, nil
// }

package collector

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectMongo collects MongoDB replica set status.
func CollectMongo(cfg *config.Config) []model.MongoCluster {
	if cfg.MongoDBIP == "" {
		return nil
	}
	if _, err := exec.LookPath("mongo"); err != nil {
		return []model.MongoCluster{{Error: "mongo CLI not available"}}
	}

	instance := fmt.Sprintf("%s:%s", cfg.MongoDBIP, cfg.MongoDBPort)
	cluster := model.MongoCluster{Instance: instance}

	args := []string{
		"-u", cfg.Creds.MongoDBUser,
		"-p", cfg.Creds.MongoDBPassword,
		"--host", cfg.MongoDBIP,
		"--port", cfg.MongoDBPort,
		"--quiet",
		"--eval", "print(JSON.stringify(rs.status()))",
	}

	out, err := exec.Command("mongo", args...).Output()
	if err != nil {
		cluster.Error = fmt.Sprintf("mongo error: %v", err)
		return []model.MongoCluster{cluster}
	}

	var rsStatus struct {
		Members []struct {
			Name           string `json:"name"`
			Health         int    `json:"health"`
			StateStr       string `json:"stateStr"`
			Uptime         int64  `json:"uptime"`
			SyncingTo      string `json:"syncingTo"`
			SyncSourceHost string `json:"syncSourceHost"`
		} `json:"members"`
	}

	if err := json.Unmarshal(out, &rsStatus); err != nil {
		cluster.Error = fmt.Sprintf("json parse error: %v", err)
		return []model.MongoCluster{cluster}
	}

	for _, m := range rsStatus.Members {
		cluster.Members = append(cluster.Members, model.MongoMember{
			Name:           m.Name,
			Health:         m.Health,
			StateStr:       m.StateStr,
			Uptime:         m.Uptime,
			SyncingTo:      m.SyncingTo,
			SyncSourceHost: m.SyncSourceHost,
		})
	}

	return []model.MongoCluster{cluster}
}

package collector

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectMongo connects to the MongoDB replica set as a single logical instance
// (URI lists every member + replicaSet=<name>) and reports each member's
// state via rs.status().
func CollectMongo(cfg *config.Config) []model.MongoCluster {
	if len(cfg.MongoDBIPs) == 0 {
		return nil
	}
	mongoBin, err := lookupMongoBinary()
	if err != nil {
		return []model.MongoCluster{{Error: err.Error()}}
	}

	// hostList: ip1:port,ip2:port,...
	hostParts := make([]string, 0, len(cfg.MongoDBIPs))
	for _, ip := range cfg.MongoDBIPs {
		hostParts = append(hostParts, ip+":"+cfg.MongoDBPort)
	}
	hostList := strings.Join(hostParts, ",")
	instance := hostList + "/?replicaSet=" + cfg.MongoRSName

	cluster := model.MongoCluster{Instance: instance}

	// mongo client URI with credentials and replicaSet.
	uri := fmt.Sprintf("mongodb://%s:%s@%s/?replicaSet=%s&authSource=admin",
		url.QueryEscape(cfg.Creds.MongoDBUser),
		url.QueryEscape(cfg.Creds.MongoDBPassword),
		hostList,
		url.QueryEscape(cfg.MongoRSName),
	)

	args := []string{
		uri,
		"--quiet",
		"--eval", "print(JSON.stringify(rs.status()))",
	}

	out, err := exec.Command(mongoBin, args...).Output()
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

// lookupMongoBinary picks `mongosh` if available (modern releases), falling
// back to `mongo` (legacy shell). Both accept the same connection-string and
// --eval flags used here.
func lookupMongoBinary() (string, error) {
	for _, bin := range []string{"mongosh", "mongo"} {
		if _, err := exec.LookPath(bin); err == nil {
			return bin, nil
		}
	}
	return "", fmt.Errorf("mongo CLI not available (neither mongosh nor mongo on PATH)")
}

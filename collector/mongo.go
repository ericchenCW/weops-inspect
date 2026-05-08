package collector

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"weops-inspect/config"
	"weops-inspect/model"
)

// CollectMongo connects to the MongoDB replica set as a single logical instance
// and reports each member's state via replSetGetStatus.
func CollectMongo(ctx context.Context, cfg *config.Config) []model.MongoCluster {
	if len(cfg.MongoDBIPs) == 0 {
		return nil
	}

	hostParts := make([]string, 0, len(cfg.MongoDBIPs))
	for _, ip := range cfg.MongoDBIPs {
		hostParts = append(hostParts, ip+":"+cfg.MongoDBPort)
	}
	hostList := strings.Join(hostParts, ",")
	instance := hostList + "/?replicaSet=" + cfg.MongoRSName

	cluster := model.MongoCluster{Instance: instance}

	uri := fmt.Sprintf("mongodb://%s:%s@%s/?replicaSet=%s&authSource=admin",
		url.QueryEscape(cfg.Creds.MongoDBUser),
		url.QueryEscape(cfg.Creds.MongoDBPassword),
		hostList,
		url.QueryEscape(cfg.MongoRSName),
	)

	p := &mongoProbe{uri: uri, cluster: &cluster}
	RunProbe(ctx, p)

	return []model.MongoCluster{cluster}
}

type mongoProbe struct {
	uri     string
	cluster *model.MongoCluster
}

func (p *mongoProbe) Name() string { return "mongo" }

func (p *mongoProbe) Run(ctx context.Context) ProbeResult {
	target := RedactDSN(p.uri)

	opts := options.Client().ApplyURI(p.uri).
		SetMaxPoolSize(2).
		SetServerSelectionTimeout(3 * time.Second).
		SetConnectTimeout(3 * time.Second).
		SetSocketTimeout(5 * time.Second)

	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		p.cluster.Error = "mongo connect error: " + RedactDSN(err.Error())
		p.cluster.ErrorClass = string(classifyMongo(err))
		return ProbeResult{Target: target, Err: WrapErr(err), ErrClass: classifyMongo(err)}
	}
	defer func() {
		// 显式 Disconnect,避免后台监控连接残留。Disconnect 自带 ctx,这里给 2s 容忍。
		dctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = client.Disconnect(dctx)
	}()

	cmd := bson.D{{Key: "replSetGetStatus", Value: 1}}
	var rsStatus struct {
		Members []struct {
			Name           string `bson:"name"`
			Health         int    `bson:"health"`
			StateStr       string `bson:"stateStr"`
			Uptime         int64  `bson:"uptime"`
			SyncingTo      string `bson:"syncingTo"`
			SyncSourceHost string `bson:"syncSourceHost"`
		} `bson:"members"`
	}
	if err := client.Database("admin").RunCommand(ctx, cmd).Decode(&rsStatus); err != nil {
		p.cluster.Error = "replSetGetStatus error: " + RedactDSN(err.Error())
		p.cluster.ErrorClass = string(classifyMongo(err))
		return ProbeResult{Target: target, Err: WrapErr(err), ErrClass: classifyMongo(err)}
	}

	for _, m := range rsStatus.Members {
		p.cluster.Members = append(p.cluster.Members, model.MongoMember{
			Name:           m.Name,
			Health:         m.Health,
			StateStr:       m.StateStr,
			Uptime:         m.Uptime,
			SyncingTo:      m.SyncingTo,
			SyncSourceHost: m.SyncSourceHost,
		})
	}

	return ProbeResult{Target: target}
}

// classifyMongo 在通用 Classify 之前识别 MongoDB 专属错误码(认证类)。
func classifyMongo(err error) ErrorClass {
	if err == nil {
		return ErrNone
	}
	var cmdErr mongo.CommandError
	if errors.As(err, &cmdErr) {
		switch cmdErr.Code {
		case 13, 18: // Unauthorized / AuthenticationFailed
			return ErrAuth
		}
		return ErrProtocol
	}
	// mongo.ServerSelectionError / network 类错误走通用判定。
	return Classify(err)
}

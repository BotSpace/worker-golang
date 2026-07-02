// Package mongox — legacy CMS Mongo (har loyiha o'z db'si: project_<id>).
package mongox

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Client struct {
	c *mongo.Client
}

func New(ctx context.Context, url string) (*Client, error) {
	c, err := mongo.Connect(ctx, options.Client().ApplyURI(url).SetMaxPoolSize(20))
	if err != nil {
		return nil, err
	}
	if err := c.Ping(ctx, nil); err != nil {
		return nil, err
	}
	return &Client{c: c}, nil
}

// ProjectUsers — project_<id> db'sidagi "users" kolleksiyasidan barcha hujjatlar.
func (m *Client) ProjectUsers(ctx context.Context, projectID int64) ([]bson.M, error) {
	coll := m.c.Database(fmt.Sprintf("project_%d", projectID)).Collection("users")
	cur, err := coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

// ProjectDataSize — project_<id> db'sidagi barcha kolleksiyalar dataSize (bytes).
// Backend _cms_bytes taxminiga mos: dbStats.dataSize ishlatamiz (aniqroq va arzon).
func (m *Client) ProjectDataSize(ctx context.Context, projectID int64) (int64, error) {
	db := m.c.Database(fmt.Sprintf("project_%d", projectID))
	var stats struct {
		DataSize int64 `bson:"dataSize"`
	}
	if err := db.RunCommand(ctx, bson.D{{Key: "dbStats", Value: 1}}).Decode(&stats); err != nil {
		return 0, err
	}
	return stats.DataSize, nil
}

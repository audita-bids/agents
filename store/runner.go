package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/audita-bids/private-kit/pkg/pb/protocols/agents"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type AsyncRunner struct {
	ID         *bson.ObjectID    `json:"id,omitempty" bson:"_id,omitempty"`
	RunnerType agents.RunnerType `json:"runner_type" bson:"runner_type"`
	RunnerID   string            `json:"runner_id" bson:"runner_id,omitempty"`
	Finished   bool              `json:"finished" bson:"finished"`
	Error      bool              `json:"error" bson:"error"`

	ErrorMessage     string     `json:"error_message,omitempty" bson:"error_message,omitempty"`
	CreatedAt        *time.Time `json:"created_at" bson:"created_at,omitempty"`
	StartedAt        *time.Time `json:"started_at" bson:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at" bson:"finished_at,omitempty"`
	PromptTokens     int64      `json:"prompt_tokens" bson:"prompt_tokens,omitempty"`
	CompletionTokens int64      `json:"completion_tokens" bson:"completion_tokens,omitempty"`
}

func (a *AsyncRunner) Marshal() []byte {
	b, _ := json.Marshal(a)

	return b
}

func (a *AsyncRunner) Unmarshal(v interface{}) error {
	b, _ := json.Marshal(a)
	return json.Unmarshal(b, &v)
}

func (a *AsyncRunner) Key() string {
	return "runner_" + a.RunnerID
}

/* Stores */
type AsyncRunnerStore struct {
	C *mongo.Collection
}

func (store *AsyncRunnerStore) CreateAsyncRunner(ctx context.Context, runner *AsyncRunner) error {
	_, err := store.C.InsertOne(ctx, runner)

	return err
}

func (store *AsyncRunnerStore) UpdateAsyncRunner(ctx context.Context, runner *AsyncRunner) error {
	_, err := store.C.ReplaceOne(ctx, bson.M{"_id": runner.ID}, runner)

	return err
}

func (store *AsyncRunnerStore) GetAsyncRunner(ctx context.Context, runner *AsyncRunner) (*AsyncRunner, error) {
	filter := &bson.M{
		"runner_id":   runner.RunnerID,
		"runner_type": runner.RunnerType,
	}

	err := store.C.FindOne(ctx, filter).
		Decode(runner)

	if err != nil {
		return nil, err
	}

	return runner, nil
}

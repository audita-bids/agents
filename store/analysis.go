package store

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Analysis struct {
	ID        *bson.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Finished  bool           `json:"finished" bson:"finished"`
	BidID     string         `json:"bid_id" bson:"process_id,omitempty"`
	UserID    string         `json:"user_id" bson:"user_id,omitempty"`
	CreatedAt *time.Time     `json:"created_at" bson:"created_at,omitempty"`
	UpdatedAt *time.Time     `json:"updated_at" bson:"updated_at,omitempty"`

	Base64           string   `json:"base64,omitempty" bson:"-"`
	Content          string   `json:"content,omitempty" bson:"content,omitempty"`
	Object           string   `json:"object,omitempty" bson:"object,omitempty"`
	Modality         string   `json:"modality,omitempty" bson:"modality,omitempty"`
	ProcessNumber    string   `json:"process_number,omitempty" bson:"process_number,omitempty"`
	EstimatedValue   string   `json:"estimated_value,omitempty" bson:"estimated_value,omitempty"`
	OpeningDate      string   `json:"opening_date,omitempty" bson:"opening_date,omitempty"`
	JudgmentCriteria string   `json:"judgment_criteria,omitempty" bson:"judgment_criteria,omitempty"`
	Keywords         []string `json:"keywords,omitempty" bson:"keywords,omitempty"`
	Score            int32    `json:"score" bson:"score"`
	AnalysisResult   string   `json:"analysis_result,omitempty" bson:"analysis_result,omitempty"`
}

func (a *Analysis) Unmarshal(v interface{}) error {
	b, _ := json.Marshal(a)
	return json.Unmarshal(b, &v)
}

func (a *Analysis) Decode(r *http.Request) {
	json.NewDecoder(r.Body).Decode(&a)
	vars := mux.Vars(r)
	id, err := bson.ObjectIDFromHex(vars["id"])
	if err != nil {
		id = bson.NewObjectID()
	}

	a.ID = &id
}

func (a *Analysis) Key() string {
	return "bid_" + a.BidID + "_" + a.UserID
}

func (a *Analysis) KeyHandles() string {
	return "bid_handles_" + a.BidID + "_" + a.UserID
}

/* Stores */
type AnalysisStore struct {
	C *mongo.Collection
}

func (store *AnalysisStore) CreateAnalysis(ctx context.Context, analysis *Analysis) error {
	_, err := store.C.InsertOne(ctx, analysis)

	return err
}

func (store *AnalysisStore) GetBidAnalysis(ctx context.Context, analysis *Analysis) (*Analysis, error) {
	filter := &bson.M{
		"process_id": analysis.BidID,
		"user_id":    analysis.UserID,
	}

	err := store.C.FindOne(ctx, filter).
		Decode(analysis)

	if err != nil {
		return nil, err
	}

	return analysis, nil
}
